package learner

import (
	"context"
	"time"

	"github.com/astraive/loza/cortex/internal/models"
	"github.com/astraive/loza/cortex/internal/storage"
)

// SalienceTracker maintains per-event-type salience scores.
// Salience indicates how predictive an event type is of successful outcomes.
type SalienceTracker struct {
	store        storage.SalienceStore
	alpha        float64 // learning rate
	defaultScore float64
}

// NewSalienceTracker creates a new salience tracker.
func NewSalienceTracker(store storage.SalienceStore) *SalienceTracker {
	return &SalienceTracker{
		store:        store,
		alpha:        0.1,
		defaultScore: 0.5,
	}
}

// WithConfig applies configuration values.
func (st *SalienceTracker) WithConfig(alpha, defaultScore float64) *SalienceTracker {
	st.alpha = alpha
	st.defaultScore = defaultScore
	return st
}

// RecordOutcome updates salience scores based on an incident outcome code.
func (st *SalienceTracker) RecordOutcome(ctx context.Context, eventTypes []string, outcomeCode int) {
	category := models.OutcomeCategory(outcomeCode)
	var reward float64
	switch category {
	case "success":
		reward = 1.0
	case "partial":
		reward = 0.5
	case "failed":
		reward = -0.5
	default:
		return
	}

	for _, eventType := range eventTypes {
		current, _ := st.store.Get(ctx, eventType)
		newScore := current + st.alpha*(reward-current)

		_ = st.store.Save(ctx, &models.SalienceScore{
			EventType:   eventType,
			Score:       newScore,
			SampleCount: 1,
			UpdatedAt:   time.Now(),
		})
	}
}

// GetSalience returns the salience score for an event type.
func (st *SalienceTracker) GetSalience(ctx context.Context, eventType string) float64 {
	score, err := st.store.Get(ctx, eventType)
	if err != nil {
		return st.defaultScore
	}
	return score
}

// GetTopSalient returns the most salient event types.
func (st *SalienceTracker) GetTopSalient(ctx context.Context, limit int) ([]*models.SalienceScore, error) {
	if limit <= 0 {
		limit = 10
	}
	return st.store.List(ctx, limit)
}
