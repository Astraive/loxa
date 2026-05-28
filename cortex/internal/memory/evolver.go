package memory

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/astraive/loxa/cortex/internal/models"
	"github.com/astraive/loxa/cortex/internal/storage"
	"github.com/astraive/loxa/cortex/internal/topology"
	"github.com/rs/zerolog/log"
)

// Config holds signature evolution parameters.
type Config struct {
	DecayPeriod        time.Duration `yaml:"decay_period"`
	DecayRate          float64       `yaml:"decay_rate"`
	ArchiveThreshold   float64       `yaml:"archive_threshold"`
	MergeTolerance     float64       `yaml:"merge_tolerance"`
	FeatureVectorAlpha float64       `yaml:"feature_vector_alpha"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		DecayPeriod:        2160 * time.Hour, // 90 days
		DecayRate:          0.1,
		ArchiveThreshold:   0.1,
		MergeTolerance:     0.3,
		FeatureVectorAlpha: 0.1,
	}
}

// FromConfig creates a Config from the cortex config file values.
func FromConfig(decayPeriod time.Duration, decayRate, archiveThreshold, mergeTolerance, featureVectorAlpha float64) Config {
	return Config{
		DecayPeriod:        decayPeriod,
		DecayRate:          decayRate,
		ArchiveThreshold:   archiveThreshold,
		MergeTolerance:     mergeTolerance,
		FeatureVectorAlpha: featureVectorAlpha,
	}
}

// SignatureEvolver handles long-horizon operational memory:
// behavioral equivalence, signature merging, decay, and cascading renames.
type SignatureEvolver struct {
	cfg           Config
	signatureStore storage.SignatureStore
	topology      *topology.Resolver
}

// NewSignatureEvolver creates a new evolver and registers topology callbacks.
func NewSignatureEvolver(cfg Config, signatureStore storage.SignatureStore, topo *topology.Resolver) *SignatureEvolver {
	ev := &SignatureEvolver{
		cfg:           cfg,
		signatureStore: signatureStore,
		topology:      topo,
	}

	// Register callback for cascading renames
	if topo != nil {
		topo.RegisterCallback(ev.onAliasRegistered)
	}

	return ev
}

// Evolve processes a new signature: checks behavioral equivalence,
// merges if similar, or creates new.
func (ev *SignatureEvolver) Evolve(ctx context.Context, sig *models.IncidentSignature) (*models.IncidentSignature, error) {
	// Compute behavioral hash
	sig.BehavioralHash = ComputeBehavioralHash(sig)

	// Check for existing behavioral match
	existing, err := ev.signatureStore.FindByBehavioralHash(ctx, sig.BehavioralHash)
	if err == nil && len(existing) > 0 {
		// Found a behaviorally equivalent signature — merge
		best := existing[0]
		for _, e := range existing[1:] {
			if e.DecayFactor > best.DecayFactor {
				best = e
			}
		}

		merged := ev.mergeSignatures(best, sig)
		if err := ev.signatureStore.Save(ctx, merged); err != nil {
			return nil, fmt.Errorf("failed to save merged signature: %w", err)
		}

		log.Info().
			Str("existing", best.SignatureID).
			Str("new", sig.SignatureID).
			Msg("Merged behaviorally equivalent signatures")

		return merged, nil
	}

	// No behavioral match — check for similar temporal patterns
	candidates, err := ev.signatureStore.List(ctx, 1000)
	if err == nil {
		for _, cand := range candidates {
			if ev.isSimilarTemporal(sig, cand) {
				merged := ev.mergeSignatures(cand, sig)
				merged.ParentSignatureID = cand.SignatureID
				merged.Version = cand.Version + 1
				if err := ev.signatureStore.Save(ctx, merged); err != nil {
					return nil, fmt.Errorf("failed to save merged signature: %w", err)
				}
				return merged, nil
			}
		}
	}

	// No match at all — create new signature
	sig.Version = 1
	sig.DecayFactor = 1.0
	now := time.Now()
	sig.LastMatchedAt = &now
	if err := ev.signatureStore.Save(ctx, sig); err != nil {
		return nil, fmt.Errorf("failed to save new signature: %w", err)
	}

	return sig, nil
}

// Decay applies time-based decay to all signatures.
// Signatures not matched in DecayPeriod have DecayFactor reduced.
func (ev *SignatureEvolver) Decay(ctx context.Context) (int, error) {
	threshold := ev.cfg.ArchiveThreshold
	archived, err := ev.signatureStore.ArchiveStale(ctx, threshold)
	if err != nil {
		return 0, fmt.Errorf("failed to archive stale signatures: %w", err)
	}

	sigs, err := ev.signatureStore.List(ctx, 10000)
	if err != nil {
		return 0, fmt.Errorf("failed to list signatures: %w", err)
	}

	now := time.Now()
	decayed := 0
	for _, sig := range sigs {
		if sig.LastMatchedAt == nil {
			continue
		}

		elapsed := now.Sub(*sig.LastMatchedAt)
		if elapsed < ev.cfg.DecayPeriod {
			continue
		}

		periods := int(elapsed / ev.cfg.DecayPeriod)
		newFactor := sig.DecayFactor * math.Pow(1.0-ev.cfg.DecayRate, float64(periods))

		if newFactor < sig.DecayFactor-0.001 {
			if err := ev.signatureStore.UpdateDecay(ctx, sig.SignatureID, newFactor); err != nil {
				log.Warn().Err(err).Str("sig", sig.SignatureID).Msg("Failed to update decay")
				continue
			}
			decayed++
		}
	}

	return archived + decayed, nil
}

// OnMatched updates the LastMatchedAt timestamp for a signature.
func (ev *SignatureEvolver) OnMatched(ctx context.Context, signatureID string) error {
	return ev.signatureStore.UpdateLastMatched(ctx, signatureID)
}

// onAliasRegistered cascades renames to all signatures referencing the old name.
func (ev *SignatureEvolver) onAliasRegistered(ctx context.Context, from, to string, validFrom time.Time) {
	sigs, err := ev.signatureStore.List(ctx, 10000)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list signatures for cascading rename")
		return
	}

	updated := 0
	for _, sig := range sigs {
		changed := false
		for i, role := range sig.ServiceRoles {
			if role == from {
				sig.ServiceRoles[i] = to
				changed = true
			}
		}
		if changed {
			sig.UpdatedAt = time.Now()
			if err := ev.signatureStore.Save(ctx, sig); err != nil {
				log.Warn().Err(err).Str("sig", sig.SignatureID).Msg("Failed to update signature for rename")
				continue
			}
			updated++
		}
	}

	if updated > 0 {
		log.Info().
			Str("from", from).
			Str("to", to).
			Int("signatures_updated", updated).
			Msg("Cascaded service rename to signatures")
	}
}

// mergeSignatures combines two signatures using incremental averaging.
func (ev *SignatureEvolver) mergeSignatures(existing, incoming *models.IncidentSignature) *models.IncidentSignature {
	alpha := ev.cfg.FeatureVectorAlpha

	merged := &models.IncidentSignature{
		SignatureID:       existing.SignatureID,
		Shape:             existing.Shape,
		ServiceRoles:      existing.ServiceRoles,
		Symptoms:          existing.Symptoms,
		TemporalPattern:   existing.TemporalPattern,
		Remediation:       existing.Remediation,
		FeatureVector:     ev.mergeVectors(existing.FeatureVector, incoming.FeatureVector, alpha),
		FeatureWeights:    existing.FeatureWeights,
		OccurrenceCount:   existing.OccurrenceCount + 1,
		AvgResolutionTime: existing.AvgResolutionTime,
		Version:           existing.Version,
		ParentSignatureID: existing.ParentSignatureID,
		DecayFactor:       math.Min(1.0, existing.DecayFactor+0.1),
		BehavioralHash:    existing.BehavioralHash,
		CreatedAt:         existing.CreatedAt,
		UpdatedAt:         time.Now(),
	}

	now := time.Now()
	merged.LastMatchedAt = &now

	// Merge remediation lists
	seen := make(map[string]bool)
	for _, r := range existing.Remediation {
		seen[r] = true
	}
	for _, r := range incoming.Remediation {
		if !seen[r] {
			merged.Remediation = append(merged.Remediation, r)
			seen[r] = true
		}
	}

	return merged
}

// mergeVectors performs incremental averaging of two feature vectors.
func (ev *SignatureEvolver) mergeVectors(existing, incoming []float64, alpha float64) []float64 {
	maxLen := len(existing)
	if len(incoming) > maxLen {
		maxLen = len(incoming)
	}

	result := make([]float64, maxLen)
	for i := 0; i < maxLen; i++ {
		var e, in float64
		if i < len(existing) {
			e = existing[i]
		}
		if i < len(incoming) {
			in = incoming[i]
		}
		result[i] = e*(1-alpha) + in*alpha
	}
	return result
}

// isSimilarTemporal checks if two signatures have similar temporal patterns.
func (ev *SignatureEvolver) isSimilarTemporal(a, b *models.IncidentSignature) bool {
	if a.Shape != b.Shape {
		return false
	}

	tol := ev.cfg.MergeTolerance
	aTP := a.TemporalPattern
	bTP := b.TemporalPattern

	if aTP.TriggerToSymptom == 0 && bTP.TriggerToSymptom == 0 {
		return true
	}
	if aTP.TriggerToSymptom == 0 || bTP.TriggerToSymptom == 0 {
		return false
	}

	ratio := float64(aTP.TriggerToSymptom) / float64(bTP.TriggerToSymptom)
	if ratio < 1.0 {
		ratio = 1.0 / ratio
	}

	return ratio-1.0 < tol
}

// ComputeBehavioralHash creates a topology-independent hash of a signature.
// Two incidents with different services but same symptom sequence produce the same hash.
func ComputeBehavioralHash(sig *models.IncidentSignature) string {
	// Sort symptoms for deterministic hashing
	symptoms := make([]string, len(sig.Symptoms))
	copy(symptoms, stringSlice(sig.Symptoms))
	sort.Strings(symptoms)

	// Build hash input: shape + sorted symptoms + temporal pattern
	parts := []string{
		sig.Shape,
		strings.Join(symptoms, ","),
		fmt.Sprintf("%d:%d:%s",
			sig.TemporalPattern.TriggerToSymptom,
			sig.TemporalPattern.SymptomDuration,
			sig.TemporalPattern.PropagationSpeed,
		),
	}

	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("%x", h[:8])
}

func stringSlice(items []models.SymptomType) []string {
	result := make([]string, len(items))
	for i, item := range items {
		result[i] = string(item)
	}
	return result
}
