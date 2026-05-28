package learner

import (
	"context"
	"fmt"
	"time"

	"github.com/astraive/loxa/cortex/internal/models"
	"github.com/astraive/loxa/cortex/internal/storage"
	"github.com/rs/zerolog/log"
)

type Learner struct {
	remediationStore storage.RemediationStore
	feedbackStore    storage.FeedbackStore
	signatureStore  storage.SignatureStore
	learningRate    float64
	weightMin       float64
	weightMax       float64
}

func NewLearner(
	remediationStore storage.RemediationStore,
	feedbackStore storage.FeedbackStore,
	signatureStore storage.SignatureStore,
) *Learner {
	return &Learner{
		remediationStore: remediationStore,
		feedbackStore:   feedbackStore,
		signatureStore:  signatureStore,
		learningRate:    0.1,
		weightMin:       0.1,
		weightMax:       2.0,
	}
}

// WithConfig applies configuration values.
func (l *Learner) WithConfig(learningRate, weightMin, weightMax float64) *Learner {
	l.learningRate = learningRate
	l.weightMin = weightMin
	l.weightMax = weightMax
	return l
}

func (l *Learner) RecordRemediation(ctx context.Context, remediation *models.Remediation) error {
	if remediation.Timestamp.IsZero() {
		remediation.Timestamp = time.Now()
	}
	return l.remediationStore.Save(ctx, remediation)
}

func (l *Learner) RecordFeedback(ctx context.Context, feedback *models.RemediationFeedback) error {
	if err := feedback.Validate(); err != nil {
		return fmt.Errorf("feedback validation failed: %w", err)
	}

	if feedback.Timestamp.IsZero() {
		feedback.Timestamp = time.Now()
	}

	if err := l.feedbackStore.Save(ctx, feedback); err != nil {
		return fmt.Errorf("failed to save feedback: %w", err)
	}

	remediation, err := l.remediationStore.Get(ctx, feedback.RemediationID)
	if err != nil {
		return nil
	}

	allFeedback, err := l.feedbackStore.GetByRemediation(ctx, feedback.RemediationID)
	if err != nil || len(allFeedback) == 0 {
		return nil
	}

	successfulCount := 0
	var totalTime int64
	for _, fb := range allFeedback {
		if fb.OutcomeCode >= 200 && fb.OutcomeCode < 300 {
			successfulCount++
		}
		totalTime += fb.TimeToResolve
	}

	if remediation.SignatureID != "" {
		sig, err := l.signatureStore.Get(ctx, remediation.SignatureID)
		if err == nil {
			sig.OccurrenceCount++

			avgTime := totalTime / int64(len(allFeedback))
			if avgTime > 0 && sig.OccurrenceCount > 1 {
				oldTotal := sig.AvgResolutionTime * int64(sig.OccurrenceCount-1)
				sig.AvgResolutionTime = (oldTotal + avgTime) / int64(sig.OccurrenceCount)
			} else if avgTime > 0 {
				sig.AvgResolutionTime = avgTime
			}

			if saveErr := l.signatureStore.Save(ctx, sig); saveErr != nil {
				log.Warn().Err(saveErr).Str("signature_id", sig.SignatureID).Msg("failed to save updated signature")
			}
		}
	}

	return nil
}

func (l *Learner) GetSuccessRate(ctx context.Context, action string, signatureID string) (float64, error) {
	return l.feedbackStore.GetSuccessRate(ctx, action, signatureID)
}

func (l *Learner) GetTopRemediations(ctx context.Context, signatureID string, limit int) ([]*models.RemediationStats, error) {
	if limit <= 0 {
		limit = 3
	}

	stats, err := l.remediationStore.ListBySignature(ctx, signatureID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get remediations: %w", err)
	}

	if stats == nil {
		return []*models.RemediationStats{}, nil
	}

	return stats, nil
}

func (l *Learner) GetAverageTimeToResolve(ctx context.Context, action string, signatureID string) (int64, error) {
	feedbacks, err := l.feedbackStore.GetByRemediation(ctx, action)
	if err != nil {
		return 0, err
	}

	if len(feedbacks) == 0 {
		return 0, nil
	}

	var totalTime int64
	for _, fb := range feedbacks {
		totalTime += fb.TimeToResolve
	}

	return totalTime / int64(len(feedbacks)), nil
}

// UpdateFeatureWeights adjusts feature vector weights based on feedback outcome.
// Successful remediations increase discriminating feature weights; failures decrease them.
func (l *Learner) UpdateFeatureWeights(ctx context.Context, signatureID string, outcomeCode int) error {
	sig, err := l.signatureStore.Get(ctx, signatureID)
	if err != nil {
		return nil // signature not found, skip
	}

	if len(sig.FeatureVector) == 0 {
		return nil
	}

	// Initialize weights if not set
	if len(sig.FeatureWeights) == 0 {
		sig.FeatureWeights = make([]float64, len(sig.FeatureVector))
		for i := range sig.FeatureWeights {
			sig.FeatureWeights[i] = 1.0
		}
	}

	category := models.OutcomeCategory(outcomeCode)
	switch category {
	case "success":
		avg := average(sig.FeatureVector)
		for i, v := range sig.FeatureVector {
			if v > avg {
				sig.FeatureWeights[i] = min(l.weightMax, sig.FeatureWeights[i]+l.learningRate)
			}
		}
	case "failed":
		avg := average(sig.FeatureVector)
		for i, v := range sig.FeatureVector {
			if v > avg {
				sig.FeatureWeights[i] = max(l.weightMin, sig.FeatureWeights[i]-l.learningRate)
			}
		}
	}

	sig.UpdatedAt = time.Now()
	return l.signatureStore.Save(ctx, sig)
}

// ReinforcePattern updates a signature after a successful remediation.
func (l *Learner) ReinforcePattern(ctx context.Context, signatureID string, remediationAction string, resolutionTime int64) error {
	sig, err := l.signatureStore.Get(ctx, signatureID)
	if err != nil {
		return nil
	}

	sig.OccurrenceCount++

	// Update average resolution time
	if resolutionTime > 0 {
		if sig.AvgResolutionTime > 0 && sig.OccurrenceCount > 1 {
			oldTotal := sig.AvgResolutionTime * int64(sig.OccurrenceCount-1)
			sig.AvgResolutionTime = (oldTotal + resolutionTime) / int64(sig.OccurrenceCount)
		} else {
			sig.AvgResolutionTime = resolutionTime
		}
	}

	// Add remediation action if not already present
	found := false
	for _, r := range sig.Remediation {
		if r == remediationAction {
			found = true
			break
		}
	}
	if !found {
		sig.Remediation = append(sig.Remediation, remediationAction)
	}

	now := time.Now()
	sig.LastMatchedAt = &now
	sig.UpdatedAt = now

	return l.signatureStore.Save(ctx, sig)
}

func average(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}