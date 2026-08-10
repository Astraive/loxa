package matcher

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/models"
)

type fakeSignatureStore struct {
	sigs      map[string]*models.IncidentSignature
	saved     []*models.IncidentSignature
	similar   map[string][]*models.SimilarIncident
	listCalls int
}

func (f *fakeSignatureStore) Save(_ context.Context, sig *models.IncidentSignature) error {
	f.saved = append(f.saved, sig)
	f.sigs[sig.SignatureID] = sig
	return nil
}

func (f *fakeSignatureStore) Get(_ context.Context, id string) (*models.IncidentSignature, error) {
	if sig, ok := f.sigs[id]; ok {
		return sig, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeSignatureStore) List(_ context.Context, limit int) ([]*models.IncidentSignature, error) {
	f.listCalls++
	var out []*models.IncidentSignature
	for _, sig := range f.sigs {
		out = append(out, sig)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeSignatureStore) FindSimilar(_ context.Context, sig *models.IncidentSignature, limit int) ([]*models.SimilarIncident, error) {
	items := f.similar[sig.SignatureID]
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (f *fakeSignatureStore) FindByBehavioralHash(_ context.Context, hash string) ([]*models.IncidentSignature, error) {
	var out []*models.IncidentSignature
	for _, sig := range f.sigs {
		if sig.BehavioralHash == hash {
			out = append(out, sig)
		}
	}
	return out, nil
}

func (f *fakeSignatureStore) UpdateDecay(_ context.Context, id string, factor float64) error {
	if sig, ok := f.sigs[id]; ok {
		sig.DecayFactor = factor
	}
	return nil
}

func (f *fakeSignatureStore) ArchiveStale(_ context.Context, threshold float64) (int, error) {
	return 0, nil
}

func (f *fakeSignatureStore) UpdateLastMatched(_ context.Context, id string) error {
	return nil
}

func TestSimilarityHelpers(t *testing.T) {
	if got := ComputeCosineSimilarity([]float64{1, 2}, []float64{1, 2}); got < 0.999 || got > 1.001 {
		t.Fatalf("expected cosine similarity close to 1, got %v", got)
	}
	if got := ComputeJaccardSimilarity([]string{"a", "b"}, []string{"a", "b"}); got != 1 {
		t.Fatalf("expected jaccard similarity 1, got %v", got)
	}
}

func TestTopKAndScore(t *testing.T) {
	store := &fakeSignatureStore{sigs: map[string]*models.IncidentSignature{}}
	m := NewSignatureMatcher(store)

	query := &models.IncidentSignature{
		SignatureID:   "q",
		FeatureVector: []float64{1, 0},
		Symptoms:      []models.SymptomType{models.SymptomTypeErrorRate},
		ServiceRoles:  []string{"svc-a"},
	}
	candidates := []*models.IncidentSignature{
		{
			SignatureID:   "low",
			FeatureVector: []float64{0, 1},
			Symptoms:      []models.SymptomType{models.SymptomTypeTimeout},
			ServiceRoles:  []string{"svc-b"},
		},
		{
			SignatureID:   "high",
			FeatureVector: []float64{1, 0},
			Symptoms:      []models.SymptomType{models.SymptomTypeErrorRate},
			ServiceRoles:  []string{"svc-a"},
		},
	}

	score, err := m.Score(query, candidates[1])
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if score < 0.89 || score > 0.91 {
		t.Fatalf("unexpected score: %v", score)
	}

	matches, err := m.TopK(query, candidates, 1)
	if err != nil {
		t.Fatalf("topk: %v", err)
	}
	if len(matches) != 1 || matches[0].Signature.SignatureID != "high" {
		t.Fatalf("unexpected top match: %+v", matches)
	}
}

func TestExtractAndRegisterSignature(t *testing.T) {
	store := &fakeSignatureStore{sigs: map[string]*models.IncidentSignature{}}
	m := NewSignatureMatcher(store)
	incident := &models.IncidentContext{
		IncidentID: "inc-1",
		CausalChain: []*models.CausalEvent{
			{Kind: "log", Timestamp: time.Now()},
			{Kind: "metric", Timestamp: time.Now().Add(30 * time.Second)},
		},
		RelatedServices: []string{"svc-a", "svc-b"},
		Symptoms: []models.Symptom{
			{Type: models.SymptomTypeErrorRate, Description: "error spike"},
		},
		Confidence: 0.5,
	}

	sig, err := m.ExtractSignature(context.Background(), incident)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if sig.Shape == "" || sig.TemporalPattern.PropagationSpeed != "fast" {
		t.Fatalf("unexpected signature: %+v", sig)
	}
	if sig.FeatureVector[0] < 1 || sig.FeatureVector[1] != 1 {
		t.Fatalf("unexpected feature vector: %+v", sig.FeatureVector)
	}

	if err := m.RegisterSignature(context.Background(), sig); err != nil {
		t.Fatalf("register: %v", err)
	}
	if sig.OccurrenceCount != 1 {
		t.Fatalf("expected occurrence count 1, got %d", sig.OccurrenceCount)
	}

	store.sigs[sig.SignatureID] = &models.IncidentSignature{
		SignatureID:       sig.SignatureID,
		OccurrenceCount:   1,
		AvgResolutionTime: 10,
	}
	if err := m.UpdateSignature(context.Background(), sig.SignatureID, &models.SignatureUpdate{
		UpdateResolutionTime: 20,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated := store.sigs[sig.SignatureID]
	if updated.OccurrenceCount != 2 || updated.AvgResolutionTime != 15 {
		t.Fatalf("unexpected update result: %+v", updated)
	}
}

func TestNewConfiguredSignatureMatcher(t *testing.T) {
	store := &fakeSignatureStore{sigs: map[string]*models.IncidentSignature{}}

	svc, err := NewConfiguredSignatureMatcher(store, config.MatcherConfig{Mode: "go"})
	if err != nil {
		t.Fatalf("go matcher should be available: %v", err)
	}
	if _, ok := svc.(*SignatureMatcher); !ok {
		t.Fatalf("expected Go-backed signature matcher, got %T", svc)
	}

	if _, err := NewConfiguredSignatureMatcher(store, config.MatcherConfig{Mode: "rust"}); !errors.Is(err, ErrRustNotAvailable) {
		t.Fatalf("expected rust matcher to be optional and unavailable, got %v", err)
	}
}

func TestFindSimilarUsesCacheAndInvalidatesOnWrite(t *testing.T) {
	store := &fakeSignatureStore{sigs: map[string]*models.IncidentSignature{
		"q": {SignatureID: "q", Shape: "deploy->timeout", Symptoms: []models.SymptomType{models.SymptomTypeTimeout}, FeatureVector: []float64{1, 1}},
		"a": {SignatureID: "a", Shape: "deploy->timeout", Symptoms: []models.SymptomType{models.SymptomTypeTimeout}, FeatureVector: []float64{1, 1}},
		"b": {SignatureID: "b", Shape: "deploy->error", Symptoms: []models.SymptomType{models.SymptomTypeErrorRate}, FeatureVector: []float64{1, 0}},
	}}

	svc, err := NewConfiguredSignatureMatcher(store, config.MatcherConfig{
		Mode:        "go",
		Parallelism: 2,
		CacheSize:   100,
		CacheTTL:    time.Minute,
	})
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}

	query := store.sigs["q"]
	first, err := svc.FindSimilar(context.Background(), query, 2)
	if err != nil {
		t.Fatalf("first find similar: %v", err)
	}
	second, err := svc.FindSimilar(context.Background(), query, 2)
	if err != nil {
		t.Fatalf("second find similar: %v", err)
	}
	if len(first) == 0 || len(second) == 0 {
		t.Fatalf("expected similar matches, got %v and %v", first, second)
	}
	if store.listCalls != 1 {
		t.Fatalf("expected cached candidate list, got %d store list calls", store.listCalls)
	}

	if err := svc.RegisterSignature(context.Background(), &models.IncidentSignature{
		SignatureID:   "c",
		Shape:         "deploy->timeout",
		Symptoms:      []models.SymptomType{models.SymptomTypeTimeout},
		FeatureVector: []float64{1, 1},
	}); err != nil {
		t.Fatalf("register signature: %v", err)
	}

	if _, err := svc.FindSimilar(context.Background(), query, 2); err != nil {
		t.Fatalf("find similar after invalidate: %v", err)
	}
	if store.listCalls != 2 {
		t.Fatalf("expected cache invalidation after write, got %d store list calls", store.listCalls)
	}
}
