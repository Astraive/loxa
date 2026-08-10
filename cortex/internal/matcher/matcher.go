package matcher

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/metrics"
	"github.com/astraive/loza/cortex/internal/models"
	"github.com/astraive/loza/cortex/internal/storage"
)

type Matcher interface {
	Score(a, b *models.IncidentSignature) (float64, error)
	TopK(query *models.IncidentSignature, candidates []*models.IncidentSignature, k int) ([]Match, error)
}

type SignatureService interface {
	Matcher
	ExtractSignature(ctx context.Context, incident *models.IncidentContext) (*models.IncidentSignature, error)
	FindSimilar(ctx context.Context, signature *models.IncidentSignature, limit int) ([]*models.SimilarIncident, error)
	RegisterSignature(ctx context.Context, signature *models.IncidentSignature) error
	UpdateSignature(ctx context.Context, signatureID string, update *models.SignatureUpdate) error
}

type Match struct {
	Signature       *models.IncidentSignature
	Similarity      float64
	MatchedSymptoms []string
	MatchedServices []string
}

func NewMatcher(store storage.SignatureStore) Matcher {
	return NewSignatureMatcher(store)
}

type MatcherType int

const (
	MatcherTypeGo MatcherType = iota
	MatcherTypeRust
)

type MatcherFactory struct {
	store storage.SignatureStore
}

func NewMatcherFactory(store storage.SignatureStore) *MatcherFactory {
	return &MatcherFactory{store: store}
}

func (f *MatcherFactory) Create(typ MatcherType) (Matcher, error) {
	switch typ {
	case MatcherTypeGo:
		return NewGoMatcher(0), nil
	case MatcherTypeRust:
		return NewRustMatcher()
	default:
		return NewGoMatcher(0), nil
	}
}

var ErrRustNotAvailable = &MatcherError{"Rust matcher not available - build with CGO_ENABLED and cortex-match crate"}

type MatcherError struct {
	msg string
}

func (e *MatcherError) Error() string {
	return e.msg
}

func CreateMatcher(store storage.SignatureStore, preferRust bool) Matcher {
	if preferRust && IsRustAvailable() {
		rustMatcher, err := NewRustMatcher()
		if err == nil {
			return rustMatcher
		}
	}
	return NewGoMatcher(0)
}

func NewConfiguredSignatureMatcher(store storage.SignatureStore, cfg config.MatcherConfig) (SignatureService, error) {
	engine, err := NewMatcherForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return NewSignatureMatcherWithEngine(store, engine, cfg), nil
}

func NewMatcherForMode(mode string) (Matcher, error) {
	return NewMatcherForConfig(config.MatcherConfig{Mode: mode})
}

func NewMatcherForConfig(cfg config.MatcherConfig) (Matcher, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
	case "", "go":
		return NewGoMatcher(cfg.Parallelism), nil
	case "rust":
		return NewRustMatcher()
	default:
		return nil, fmt.Errorf("unsupported matcher mode %q", cfg.Mode)
	}
}

type RustMatcher struct{}

func NewRustMatcher() (Matcher, error) {
	if !IsRustAvailable() {
		return nil, ErrRustNotAvailable
	}
	return &RustMatcher{}, nil
}

type SignatureMatcher struct {
	signatureStore storage.SignatureStore
	engine         Matcher
	cacheTTL       time.Duration
	cacheSize      int
	cacheMu        sync.RWMutex
	cacheLoadedAt  time.Time
	cache          []*models.IncidentSignature
}

func NewSignatureMatcher(store storage.SignatureStore) *SignatureMatcher {
	return NewSignatureMatcherWithEngine(store, NewGoMatcher(0), config.MatcherConfig{
		CacheSize: 1024,
		CacheTTL:  30 * time.Second,
	})
}

func NewSignatureMatcherWithEngine(store storage.SignatureStore, engine Matcher, cfg config.MatcherConfig) *SignatureMatcher {
	if engine == nil {
		engine = NewGoMatcher(cfg.Parallelism)
	}
	if cfg.CacheSize == 0 {
		cfg.CacheSize = 1024
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 30 * time.Second
	}
	return &SignatureMatcher{
		signatureStore: store,
		engine:         engine,
		cacheTTL:       cfg.CacheTTL,
		cacheSize:      cfg.CacheSize,
	}
}

func (m *SignatureMatcher) Score(a, b *models.IncidentSignature) (float64, error) {
	return m.engine.Score(a, b)
}

func (m *SignatureMatcher) TopK(query *models.IncidentSignature, candidates []*models.IncidentSignature, k int) ([]Match, error) {
	return m.engine.TopK(query, candidates, k)
}

type GoMatcher struct {
	parallelism int
}

func NewGoMatcher(parallelism int) Matcher {
	if parallelism <= 0 {
		parallelism = runtime.GOMAXPROCS(0)
		if parallelism <= 0 {
			parallelism = 1
		}
	}
	return &GoMatcher{parallelism: parallelism}
}

func (m *GoMatcher) Score(a, b *models.IncidentSignature) (float64, error) {
	if a == nil || b == nil {
		return 0, nil
	}

	wFeature := 0.4
	wSymptom := 0.4
	wTemporal := 0.2

	featureSim := cosineSimilarity(a.FeatureVector, b.FeatureVector)
	symptomSim := jaccardSimilarity(stringSlice(a.Symptoms), stringSlice(b.Symptoms))
	temporalSim := 0.5

	return wFeature*featureSim + wSymptom*symptomSim + wTemporal*temporalSim, nil
}

// ScoreTopologyIndependent scores two signatures without using service names.
// Uses only symptom classes, temporal patterns, and resolution patterns.
func (m *GoMatcher) ScoreTopologyIndependent(a, b *models.IncidentSignature) (float64, error) {
	if a == nil || b == nil {
		return 0, nil
	}

	wSymptom := 0.5
	wTemporal := 0.3
	wResolution := 0.2

	symptomSim := jaccardSimilarity(stringSlice(a.Symptoms), stringSlice(b.Symptoms))
	temporalSim := computeTemporalSimilarity(a.TemporalPattern, b.TemporalPattern)
	resolutionSim := computeResolutionSimilarity(a, b)

	return wSymptom*symptomSim + wTemporal*temporalSim + wResolution*resolutionSim, nil
}

// serviceRole maps a service name to a generic role based on naming conventions.
func serviceRole(name string) string {
	lower := strings.ToLower(name)

	rolePatterns := map[string][]string{
		"database": {"db", "database", "postgres", "mysql", "mongo", "redis", "dynamodb", "cockroach"},
		"cache":    {"cache", "redis", "memcached", "memcache"},
		"queue":    {"queue", "kafka", "rabbit", "sqs", "pubsub", "broker", "event"},
		"api":      {"api", "gateway", "proxy", "edge", "frontend", "web", "app"},
		"worker":   {"worker", "job", "task", "cron", "scheduler", "processor"},
		"storage":  {"storage", "s3", "blob", "file", "blob", "gcs"},
		"auth":     {"auth", "login", "oauth", "identity", "iam"},
		"search":   {"search", "elastic", "solr", "index"},
		"analytics": {"analytics", "metric", "monitor", "telemetry", "trace"},
		"notification": {"notification", "email", "sms", "push", "alert"},
		"shipping": {"shipping", "delivery", "logistics", "tracking"},
		"payment":  {"payment", "billing", "checkout", "stripe", "paypal"},
		"inventory": {"inventory", "stock", "catalog", "product"},
		"user":     {"user", "profile", "account", "member", "customer"},
	}

	for role, patterns := range rolePatterns {
		for _, pattern := range patterns {
			if strings.Contains(lower, pattern) {
				return role
			}
		}
	}

	return "service"
}

// computeTemporalSimilarity compares two temporal patterns.
func computeTemporalSimilarity(a, b models.TemporalPattern) float64 {
	if a.TriggerToSymptom == 0 && b.TriggerToSymptom == 0 {
		return 1.0
	}
	if a.TriggerToSymptom == 0 || b.TriggerToSymptom == 0 {
		return 0.0
	}

	// Compare propagation speeds
	if a.PropagationSpeed == b.PropagationSpeed {
		return 0.8
	}

	// Compare durations with tolerance
	maxDur := math.Max(float64(a.SymptomDuration), float64(b.SymptomDuration))
	if maxDur == 0 {
		return 0.5
	}
	diff := math.Abs(float64(a.SymptomDuration - b.SymptomDuration))
	return math.Max(0, 1.0-diff/maxDur)
}

// computeResolutionSimilarity compares resolution patterns between signatures.
func computeResolutionSimilarity(a, b *models.IncidentSignature) float64 {
	if len(a.Remediation) == 0 && len(b.Remediation) == 0 {
		return 1.0
	}
	if len(a.Remediation) == 0 || len(b.Remediation) == 0 {
		return 0.0
	}

	// Jaccard similarity of remediation actions
	setA := make(map[string]bool)
	for _, r := range a.Remediation {
		setA[r] = true
	}
	setB := make(map[string]bool)
	for _, r := range b.Remediation {
		setB[r] = true
	}

	intersection := 0
	for r := range setA {
		if setB[r] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

func (m *GoMatcher) TopK(query *models.IncidentSignature, candidates []*models.IncidentSignature, k int) ([]Match, error) {
	if k <= 0 {
		k = 5
	}

	type scored struct {
		sig      *models.IncidentSignature
		sim      float64
		symptoms []string
		services []string
	}

	results := make([]scored, len(candidates))
	workerCount := m.parallelism
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > len(candidates) {
		workerCount = len(candidates)
	}
	if workerCount == 0 {
		workerCount = 1
	}

	indexCh := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexCh {
				cand := candidates[idx]
				sim, _ := m.Score(query, cand)
				results[idx] = scored{
					sig:      cand,
					sim:      sim,
					symptoms: intersection(stringSlice(query.Symptoms), stringSlice(cand.Symptoms)),
					services: intersection(query.ServiceRoles, cand.ServiceRoles),
				}
			}
		}()
	}
	for i := range candidates {
		indexCh <- i
	}
	close(indexCh)
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].sim > results[j].sim
	})

	if k > len(results) {
		k = len(results)
	}

	matches := make([]Match, k)
	for i := 0; i < k; i++ {
		matches[i] = Match{
			Signature:       results[i].sig,
			Similarity:      results[i].sim,
			MatchedSymptoms: results[i].symptoms,
			MatchedServices: results[i].services,
		}
	}

	return matches, nil
}

func (m *RustMatcher) Score(a, b *models.IncidentSignature) (float64, error) {
	return 0, ErrRustNotAvailable
}

func (m *RustMatcher) TopK(query *models.IncidentSignature, candidates []*models.IncidentSignature, k int) ([]Match, error) {
	return nil, ErrRustNotAvailable
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	dot := 0.0
	aMag := 0.0
	bMag := 0.0

	for i := 0; i < minLen; i++ {
		dot += a[i] * b[i]
		aMag += a[i] * a[i]
		bMag += b[i] * b[i]
	}

	if aMag == 0 || bMag == 0 {
		return 0
	}

	sim := dot / (math.Sqrt(aMag) * math.Sqrt(bMag))
	if sim < 0 {
		return 0
	}
	if sim > 1 {
		return 1
	}
	return sim
}

func jaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}

	setA := make(map[string]bool)
	setB := make(map[string]bool)

	for _, s := range a {
		setA[s] = true
	}
	for _, s := range b {
		setB[s] = true
	}

	intersectionCount := 0
	union := make(map[string]struct{}, len(setA)+len(setB))
	for k := range setA {
		union[k] = struct{}{}
	}
	for k := range setB {
		union[k] = struct{}{}
		if setA[k] {
			intersectionCount++
		}
	}

	if len(union) == 0 {
		return 0
	}

	return float64(intersectionCount) / float64(len(union))
}

func stringSlice(symptoms []models.SymptomType) []string {
	result := make([]string, len(symptoms))
	for i, s := range symptoms {
		result[i] = string(s)
	}
	return result
}

func intersection(a, b []string) []string {
	setB := make(map[string]bool)
	for _, s := range b {
		setB[s] = true
	}

	var result []string
	for _, s := range a {
		if setB[s] {
			result = append(result, s)
		}
	}
	return result
}

func (m *SignatureMatcher) ExtractSignature(ctx context.Context, incident *models.IncidentContext) (*models.IncidentSignature, error) {
	sig := &models.IncidentSignature{
		SignatureID: incident.IncidentID,
		CreatedAt:   time.Now(),
	}

	sig.Shape = m.extractShape(incident)
	sig.ServiceRoles = make([]string, len(incident.RelatedServices))
	for i, svc := range incident.RelatedServices {
		sig.ServiceRoles[i] = serviceRole(svc)
	}
	sig.Symptoms = m.extractSymptomTypes(incident.Symptoms)
	sig.TemporalPattern = m.extractTemporalPattern(incident)
	sig.FeatureVector = m.computeFeatureVector(incident)

	return sig, nil
}

func (m *SignatureMatcher) FindSimilar(ctx context.Context, signature *models.IncidentSignature, limit int) ([]*models.SimilarIncident, error) {
	if limit <= 0 {
		limit = 5
	}

	candidates, err := m.loadCandidates(ctx)
	if err != nil {
		metrics.MatcherErrors.Inc()
		similar, fallbackErr := m.signatureStore.FindSimilar(ctx, signature, limit)
		if fallbackErr != nil {
			return nil, fmt.Errorf("failed to find similar: %w", fallbackErr)
		}
		return similar, nil
	}

	filtered := make([]*models.IncidentSignature, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil || candidate.SignatureID == signature.SignatureID {
			continue
		}
		filtered = append(filtered, candidate)
	}
	metrics.MatcherCandidateBatch.Observe(float64(len(filtered)))

	matches, err := m.engine.TopK(signature, filtered, limit)
	if err != nil {
		metrics.MatcherErrors.Inc()
		return nil, fmt.Errorf("failed to score candidates: %w", err)
	}

	similar := make([]*models.SimilarIncident, 0, len(matches))
	for _, match := range matches {
		if match.Signature == nil || match.Similarity < 0.5 {
			continue
		}
		resolution := ""
		if len(match.Signature.Remediation) > 0 {
			resolution = match.Signature.Remediation[0]
		}
		similar = append(similar, &models.SimilarIncident{
			IncidentID:     match.Signature.SignatureID,
			Similarity:     match.Similarity,
			Shape:          match.Signature.Shape,
			Resolution:     resolution,
			ResolutionTime: match.Signature.AvgResolutionTime,
			SuccessRate:    0,
		})
	}

	return similar, nil
}

func (m *SignatureMatcher) RegisterSignature(ctx context.Context, signature *models.IncidentSignature) error {
	signature.UpdatedAt = time.Now()
	if signature.OccurrenceCount == 0 {
		signature.OccurrenceCount = 1
	}
	if err := m.signatureStore.Save(ctx, signature); err != nil {
		return err
	}
	m.invalidateCache()
	return nil
}

func (m *SignatureMatcher) UpdateSignature(ctx context.Context, signatureID string, update *models.SignatureUpdate) error {
	sig, err := m.signatureStore.Get(ctx, signatureID)
	if err != nil {
		return fmt.Errorf("signature not found: %w", err)
	}

	if update.IncrementOccurrence {
		sig.OccurrenceCount++
	}

	if len(update.AddRemediation) > 0 {
		sig.Remediation = append(sig.Remediation, update.AddRemediation...)
	}

	if update.UpdateResolutionTime > 0 {
		oldTotal := sig.AvgResolutionTime * int64(sig.OccurrenceCount)
		sig.OccurrenceCount++
		sig.AvgResolutionTime = (oldTotal + update.UpdateResolutionTime) / int64(sig.OccurrenceCount)
	}

	sig.UpdatedAt = time.Now()
	if err := m.signatureStore.Save(ctx, sig); err != nil {
		return err
	}
	m.invalidateCache()
	return nil
}

func (m *SignatureMatcher) loadCandidates(ctx context.Context) ([]*models.IncidentSignature, error) {
	now := time.Now()

	m.cacheMu.RLock()
	if len(m.cache) > 0 && now.Sub(m.cacheLoadedAt) < m.cacheTTL {
		cached := append([]*models.IncidentSignature(nil), m.cache...)
		m.cacheMu.RUnlock()
		metrics.MatcherCacheRequests.WithLabelValues("hit").Inc()
		return cached, nil
	}
	m.cacheMu.RUnlock()

	metrics.MatcherCacheRequests.WithLabelValues("miss").Inc()
	candidates, err := m.signatureStore.List(ctx, m.cacheSize)
	if err != nil {
		return nil, err
	}

	m.cacheMu.Lock()
	m.cache = append([]*models.IncidentSignature(nil), candidates...)
	m.cacheLoadedAt = now
	m.cacheMu.Unlock()

	return append([]*models.IncidentSignature(nil), candidates...), nil
}

func (m *SignatureMatcher) invalidateCache() {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	m.cache = nil
	m.cacheLoadedAt = time.Time{}
}

func (m *SignatureMatcher) extractShape(incident *models.IncidentContext) string {
	var parts []string

	for _, event := range incident.CausalChain {
		parts = append(parts, event.Kind)
	}

	for _, symptom := range incident.Symptoms {
		if symptom.Description != "" {
			parts = append(parts, string(symptom.Type))
		}
	}

	if len(parts) == 0 {
		return "unknown"
	}

	return strings.Join(parts, "→")
}

func (m *SignatureMatcher) extractSymptomTypes(symptoms []models.Symptom) []models.SymptomType {
	types := make(map[models.SymptomType]bool)
	var result []models.SymptomType

	for _, s := range symptoms {
		if !types[s.Type] {
			types[s.Type] = true
			result = append(result, s.Type)
		}
	}

	return result
}

func (m *SignatureMatcher) extractTemporalPattern(incident *models.IncidentContext) models.TemporalPattern {
	if len(incident.CausalChain) < 2 {
		return models.TemporalPattern{PropagationSpeed: "unknown"}
	}

	first := incident.CausalChain[0].Timestamp
	last := incident.CausalChain[len(incident.CausalChain)-1].Timestamp
	duration := last.Sub(first).Milliseconds()

	speed := "medium"
	if duration < 60000 {
		speed = "fast"
	} else if duration > 300000 {
		speed = "slow"
	}

	triggerToSymptom := int64(0)
	for _, event := range incident.CausalChain {
		if event.Timestamp.After(first) {
			triggerToSymptom = event.Timestamp.Sub(first).Milliseconds()
			break
		}
	}

	return models.TemporalPattern{
		TriggerToSymptom: triggerToSymptom,
		SymptomDuration:  duration,
		PropagationSpeed: speed,
	}
}

func (m *SignatureMatcher) computeFeatureVector(incident *models.IncidentContext) []float64 {
	vector := make([]float64, 10)

	vector[0] = float64(len(incident.CausalChain))
	vector[1] = float64(len(incident.Symptoms))

	hasLatency := false
	hasError := false
	hasTimeout := false
	for _, s := range incident.Symptoms {
		switch s.Type {
		case models.SymptomTypeLatencySpike:
			hasLatency = true
		case models.SymptomTypeErrorRate:
			hasError = true
		case models.SymptomTypeTimeout:
			hasTimeout = true
		}
	}

	if hasLatency {
		vector[2] = 1.0
	}
	if hasError {
		vector[3] = 1.0
	}
	if hasTimeout {
		vector[4] = 1.0
	}

	uniqueServices := make(map[string]bool)
	for _, s := range incident.RelatedServices {
		uniqueServices[s] = true
	}
	vector[5] = float64(len(uniqueServices))

	if len(incident.SimilarIncidents) > 0 {
		vector[6] = incident.SimilarIncidents[0].Similarity
	}

	vector[7] = incident.Confidence
	vector[8] = float64(len(incident.SuggestedActions))
	vector[9] = 1.0

	for i := range vector {
		if vector[i] > 1.0 {
			vector[i] = 1.0
		}
	}

	return vector
}

func ComputeCosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	dotProduct := 0.0
	aMag := 0.0
	bMag := 0.0

	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		dotProduct += a[i] * b[i]
		aMag += a[i] * a[i]
		bMag += b[i] * b[i]
	}

	if aMag == 0 || bMag == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(aMag) * math.Sqrt(bMag))
}

func ComputeJaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0.0
	}

	setA := make(map[string]bool)
	setB := make(map[string]bool)

	for _, s := range a {
		setA[s] = true
	}
	for _, s := range b {
		setB[s] = true
	}

	intersectionCount := 0
	union := make(map[string]bool, len(setA)+len(setB))
	for k := range setA {
		union[k] = true
	}
	for k := range setB {
		union[k] = true
		if setA[k] {
			intersectionCount++
		}
	}

	if len(union) == 0 {
		return 0.0
	}

	return float64(intersectionCount) / float64(len(union))
}
