package learner

import (
	"context"
	"errors"
	"testing"

	"github.com/astraive/loza/cortex/internal/models"
)

type fakeRemediationStore struct {
	remediations map[string]*models.Remediation
	stats        map[string][]*models.RemediationStats
	saved        []*models.Remediation
}

func (f *fakeRemediationStore) Save(_ context.Context, rem *models.Remediation) error {
	f.saved = append(f.saved, rem)
	f.remediations[rem.RemediationID] = rem
	return nil
}

func (f *fakeRemediationStore) Get(_ context.Context, id string) (*models.Remediation, error) {
	if rem, ok := f.remediations[id]; ok {
		return rem, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeRemediationStore) ListByIncident(context.Context, string) ([]*models.Remediation, error) {
	return nil, nil
}

func (f *fakeRemediationStore) ListBySignature(_ context.Context, signatureID string, limit int) ([]*models.RemediationStats, error) {
	items := f.stats[signatureID]
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

type fakeFeedbackStore struct {
	feedbackByRemediation map[string][]*models.RemediationFeedback
	successRates          map[string]float64
	saved                 []*models.RemediationFeedback
}

func (f *fakeFeedbackStore) Save(_ context.Context, fb *models.RemediationFeedback) error {
	f.saved = append(f.saved, fb)
	f.feedbackByRemediation[fb.RemediationID] = append(f.feedbackByRemediation[fb.RemediationID], fb)
	return nil
}

func (f *fakeFeedbackStore) GetByRemediation(_ context.Context, remediationID string) ([]*models.RemediationFeedback, error) {
	return f.feedbackByRemediation[remediationID], nil
}

func (f *fakeFeedbackStore) GetSuccessRate(_ context.Context, action string, signatureID string) (float64, error) {
	return f.successRates[action+"|"+signatureID], nil
}

type fakeSignatureStoreForLearner struct {
	sigs map[string]*models.IncidentSignature
	saved []*models.IncidentSignature
}

func (f *fakeSignatureStoreForLearner) Save(_ context.Context, sig *models.IncidentSignature) error {
	f.saved = append(f.saved, sig)
	f.sigs[sig.SignatureID] = sig
	return nil
}

func (f *fakeSignatureStoreForLearner) Get(_ context.Context, id string) (*models.IncidentSignature, error) {
	if sig, ok := f.sigs[id]; ok {
		return sig, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeSignatureStoreForLearner) List(context.Context, int) ([]*models.IncidentSignature, error) {
	return nil, nil
}

func (f *fakeSignatureStoreForLearner) FindSimilar(context.Context, *models.IncidentSignature, int) ([]*models.SimilarIncident, error) {
	return nil, nil
}

func (f *fakeSignatureStoreForLearner) FindByBehavioralHash(context.Context, string) ([]*models.IncidentSignature, error) {
	return nil, nil
}

func (f *fakeSignatureStoreForLearner) UpdateDecay(context.Context, string, float64) error {
	return nil
}

func (f *fakeSignatureStoreForLearner) ArchiveStale(context.Context, float64) (int, error) {
	return 0, nil
}

func (f *fakeSignatureStoreForLearner) UpdateLastMatched(context.Context, string) error {
	return nil
}

func TestRecordRemediationAndFeedback(t *testing.T) {
	remStore := &fakeRemediationStore{
		remediations: map[string]*models.Remediation{
			"rem-1": {RemediationID: "rem-1", SignatureID: "sig-1"},
		},
		stats: map[string][]*models.RemediationStats{
			"sig-1": {
				{Action: "restart", SuccessRate: 0.9, AvgTimeToResolve: 42},
				{Action: "rollback", SuccessRate: 0.8, AvgTimeToResolve: 55},
			},
		},
	}
	fbStore := &fakeFeedbackStore{
		feedbackByRemediation: map[string][]*models.RemediationFeedback{
			"rem-1": {
				{RemediationID: "rem-1", OutcomeCode: 200, OutcomeCategory: "success", TimeToResolve: 20},
				{RemediationID: "rem-1", OutcomeCode: 500, OutcomeCategory: "failed", TimeToResolve: 40},
			},
		},
		successRates: map[string]float64{"restart|sig-1": 0.9},
	}
	sigStore := &fakeSignatureStoreForLearner{
		sigs: map[string]*models.IncidentSignature{
			"sig-1": {SignatureID: "sig-1", OccurrenceCount: 1, AvgResolutionTime: 10},
		},
	}

	l := NewLearner(remStore, fbStore, sigStore)

	remediation := &models.Remediation{RemediationID: "rem-2"}
	if err := l.RecordRemediation(context.Background(), remediation); err != nil {
		t.Fatalf("record remediation: %v", err)
	}
	if remediation.Timestamp.IsZero() {
		t.Fatal("expected remediation timestamp to be set")
	}

	feedback := &models.RemediationFeedback{RemediationID: "rem-1", IncidentID: "inc-1", OutcomeCode: 200, OutcomeCategory: "success"}
	if err := l.RecordFeedback(context.Background(), feedback); err != nil {
		t.Fatalf("record feedback: %v", err)
	}
	if feedback.Timestamp.IsZero() {
		t.Fatal("expected feedback timestamp to be set")
	}
	if sigStore.sigs["sig-1"].OccurrenceCount != 2 {
		t.Fatalf("expected signature occurrence count to be updated, got %d", sigStore.sigs["sig-1"].OccurrenceCount)
	}
	if sigStore.sigs["sig-1"].AvgResolutionTime != 15 {
		t.Fatalf("expected signature avg time to be updated, got %d", sigStore.sigs["sig-1"].AvgResolutionTime)
	}

	stats, err := l.GetTopRemediations(context.Background(), "sig-1", 0)
	if err != nil {
		t.Fatalf("top remediations: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected stats, got %+v", stats)
	}

	rate, err := l.GetSuccessRate(context.Background(), "restart", "sig-1")
	if err != nil {
		t.Fatalf("success rate: %v", err)
	}
	if rate != 0.9 {
		t.Fatalf("unexpected success rate: %v", rate)
	}
}
