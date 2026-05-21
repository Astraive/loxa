package reconstructor

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/astraive/loxa/loxa-cortex/internal/graph"
	"github.com/astraive/loxa/loxa-cortex/internal/learner"
	"github.com/astraive/loxa/loxa-cortex/internal/matcher"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
	"github.com/astraive/loxa/loxa-cortex/internal/storage"
)

type IncidentReconstructor struct {
	graphBuilder  *graph.Builder
	matcher       matcher.SignatureService
	remediation   *learner.Learner
	incidentStore storage.IncidentStore
	fastDepth     int
	fastEvents    int
	fastWindow    time.Duration
	deepDepth     int
	deepEvents    int
	deepWindow    time.Duration
	confidence    ConfidenceWeights
}

type ConfidenceWeights struct {
	CausalChainBonus  float64
	SymptomBonus      float64
	SimilarityWeight  float64
	RemediationWeight float64
	MaxConfidence     float64
	MinConfidence     float64
}

func NewIncidentReconstructor(
	graphBuilder *graph.Builder,
	matcher matcher.SignatureService,
	remediation *learner.Learner,
	incidentStore storage.IncidentStore,
) *IncidentReconstructor {
	return &IncidentReconstructor{
		graphBuilder:  graphBuilder,
		matcher:       matcher,
		remediation:   remediation,
		incidentStore: incidentStore,
		fastDepth:     3,
		fastEvents:    20,
		fastWindow:    30 * time.Minute,
		deepDepth:     10,
		deepEvents:    200,
		deepWindow:    2 * time.Hour,
		confidence: ConfidenceWeights{
			CausalChainBonus:  0.1,
			SymptomBonus:      0.1,
			SimilarityWeight:  0.1,
			RemediationWeight: 0.1,
			MaxConfidence:     1.0,
			MinConfidence:     0.0,
		},
	}
}

// WithConfig applies configuration values from the config file.
func (r *IncidentReconstructor) WithConfig(fastDepth, fastEvents int, fastWindow time.Duration, deepDepth, deepEvents int, deepWindow time.Duration, cw ConfidenceWeights) *IncidentReconstructor {
	r.fastDepth = fastDepth
	r.fastEvents = fastEvents
	r.fastWindow = fastWindow
	r.deepDepth = deepDepth
	r.deepEvents = deepEvents
	r.deepWindow = deepWindow
	r.confidence = cw
	return r
}

func (r *IncidentReconstructor) ReconstructFast(ctx context.Context, incidentID string) (*models.IncidentContext, error) {
	return r.reconstructAdaptive(ctx, incidentID, r.fastDepth, r.fastEvents, r.fastWindow)
}

func (r *IncidentReconstructor) ReconstructDeep(ctx context.Context, incidentID string) (*models.IncidentContext, error) {
	return r.reconstructAdaptive(ctx, incidentID, r.deepDepth, r.deepEvents, r.deepWindow)
}

func (r *IncidentReconstructor) ReconstructWithMode(ctx context.Context, incidentID string, mode string) (*models.IncidentContext, error) {
	switch mode {
	case "deep":
		return r.reconstructAdaptive(ctx, incidentID, r.deepDepth, r.deepEvents, r.deepWindow)
	default: // "fast"
		return r.reconstructAdaptive(ctx, incidentID, r.fastDepth, r.fastEvents, r.fastWindow)
	}
}

func (r *IncidentReconstructor) ReconstructFromSignal(ctx context.Context, signal *models.IncidentSignal) (*models.IncidentContext, error) {
	incidentID := signal.SignalID

	incident := &models.Incident{
		ID:             incidentID,
		Timestamp:      signal.Timestamp,
		PrimaryService: signal.Service,
		Status:         "active",
		Severity:       signal.Severity,
		CreatedAt:      time.Now(),
	}
	if err := r.incidentStore.Save(ctx, incident); err != nil {
		return nil, fmt.Errorf("failed to save incident: %w", err)
	}

	return r.reconstruct(ctx, incidentID, 3)
}

func (r *IncidentReconstructor) reconstruct(ctx context.Context, incidentID string, maxDepth int) (*models.IncidentContext, error) {
	incident, err := r.incidentStore.Get(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("incident %s not found: %w", incidentID, err)
	}

	graphView, err := r.graphBuilder.TraverseGraph(ctx, incidentID, models.TraversalOptions{
		MaxDepth: maxDepth,
		EdgeTypes: []models.EdgeType{
			models.EdgeTypeSameIncident,
			models.EdgeTypeCausedProbably,
			models.EdgeTypeDeployedBefore,
			models.EdgeTypeMetricSpikedAfter,
			models.EdgeTypeLogErrorAfter,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to traverse graph: %w", err)
	}

	context := &models.IncidentContext{
		IncidentID: incidentID,
	}

	if incident != nil {
		context.Timestamp = incident.Timestamp
	}

	causalChain := r.buildCausalChain(graphView)
	sort.Slice(causalChain, func(i, j int) bool {
		return causalChain[i].Timestamp.Before(causalChain[j].Timestamp)
	})
	context.CausalChain = causalChain

	relatedServices := make(map[string]bool)
	for _, event := range causalChain {
		relatedServices[event.Service] = true
	}
	for service := range relatedServices {
		context.RelatedServices = append(context.RelatedServices, service)
	}

	symptoms := r.extractSymptoms(causalChain)
	context.Symptoms = symptoms

	sig, err := r.matcher.ExtractSignature(ctx, context)
	if err == nil {
		if incident != nil && incident.SignatureID != "" {
			sig.SignatureID = incident.SignatureID
		}
		similar, err := r.matcher.FindSimilar(ctx, sig, 5)
		if err == nil {
			context.SimilarIncidents = similar
		}
	}
	if context.SimilarIncidents == nil {
		context.SimilarIncidents = []*models.SimilarIncident{}
	}

	if len(context.SimilarIncidents) > 5 {
		context.SimilarIncidents = context.SimilarIncidents[:5]
	}
	sort.Slice(context.SimilarIncidents, func(i, j int) bool {
		return context.SimilarIncidents[i].Similarity > context.SimilarIncidents[j].Similarity
	})

	if len(context.SimilarIncidents) > 0 {
		stats, err := r.remediation.GetTopRemediations(ctx, context.SimilarIncidents[0].IncidentID, 3)
		if err == nil {
			for i, s := range stats {
				context.SuggestedActions = append(context.SuggestedActions, models.RemediationAction{
					Action:           s.Action,
					SuccessRate:      s.SuccessRate,
					AvgTimeToResolve: s.AvgTimeToResolve,
					Priority:         i + 1,
				})
			}
		}
	}

	if len(context.SuggestedActions) > 3 {
		context.SuggestedActions = context.SuggestedActions[:3]
	}
	sort.Slice(context.SuggestedActions, func(i, j int) bool {
		return context.SuggestedActions[i].Priority < context.SuggestedActions[j].Priority
	})

	confidence := r.computeConfidence(context)
	context.Confidence = confidence

	report := r.generateExplainReport(context)
	context.Explain = report.Narrative
	context.ExplainReport = report

	return context, nil
}

func (r *IncidentReconstructor) generateExplainReport(ctx *models.IncidentContext) *models.ExplainReport {
	if ctx == nil {
		return &models.ExplainReport{}
	}

	report := &models.ExplainReport{}

	// Build narrative
	var parts []string
	parts = append(parts, fmt.Sprintf("Incident %s analysis:", ctx.IncidentID))

	if len(ctx.CausalChain) > 0 {
		services := make(map[string]bool)
		for _, e := range ctx.CausalChain {
			if e.Service != "" {
				services[e.Service] = true
			}
		}
		parts = append(parts, fmt.Sprintf("Causal chain of %d events across %d services.", len(ctx.CausalChain), len(services)))
	}

	if len(ctx.Symptoms) > 0 {
		symptomTypes := make(map[string]int)
		for _, s := range ctx.Symptoms {
			symptomTypes[string(s.Type)]++
		}
		var desc []string
		for t, count := range symptomTypes {
			desc = append(desc, fmt.Sprintf("%s(%d)", t, count))
		}
		parts = append(parts, fmt.Sprintf("Symptoms detected: %d total (%v).", len(ctx.Symptoms), desc))
	}

	if len(ctx.SimilarIncidents) > 0 {
		best := ctx.SimilarIncidents[0]
		parts = append(parts, fmt.Sprintf("Most similar past incident: %s (similarity: %.2f).", best.IncidentID, best.Similarity))
		if best.Shape != "" {
			parts = append(parts, fmt.Sprintf("Matching pattern: %s.", best.Shape))
		}
	}

	if len(ctx.SuggestedActions) > 0 {
		best := ctx.SuggestedActions[0]
		parts = append(parts, fmt.Sprintf("Top remediation: \"%s\" (historical success rate: %.0f%%, avg resolution: %ds).",
			best.Action, best.SuccessRate*100, best.AvgTimeToResolve))
	}

	parts = append(parts, fmt.Sprintf("Overall confidence: %.0f%%.", ctx.Confidence*100))
	report.Narrative = joinParts(parts)

	// Confidence breakdown
	chainStrength := math.Min(1.0, float64(len(ctx.CausalChain))/10.0)
	symptomCoverage := math.Min(1.0, float64(len(ctx.Symptoms))/3.0)
	historicalMatch := 0.0
	if len(ctx.SimilarIncidents) > 0 {
		historicalMatch = ctx.SimilarIncidents[0].Similarity
	}
	remediationConf := 0.0
	if len(ctx.SuggestedActions) > 0 {
		remediationConf = ctx.SuggestedActions[0].SuccessRate
	}
	report.ConfidenceBreakdown = models.ConfidenceBreakdown{
		CausalChainStrength:   chainStrength,
		SymptomCoverage:       symptomCoverage,
		HistoricalMatch:       historicalMatch,
		RemediationConfidence: remediationConf,
	}

	// Key findings: top 3 highest signal-density events
	sorted := make([]*models.CausalEvent, len(ctx.CausalChain))
	copy(sorted, ctx.CausalChain)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].SignalDensity > sorted[j].SignalDensity
	})
	for i, e := range sorted {
		if i >= 3 {
			break
		}
		report.KeyFindings = append(report.KeyFindings, fmt.Sprintf("[%s] %s on %s (signal: %.2f)", e.Kind, e.Description, e.Service, e.SignalDensity))
	}

	// Data gaps
	hasDeploy := false
	hasMetric := false
	for _, e := range ctx.CausalChain {
		switch e.Kind {
		case "deployment", "deploy":
			hasDeploy = true
		case "metric":
			hasMetric = true
		case "log":
			_ = true
		}
	}
	if !hasDeploy && len(ctx.Symptoms) > 0 {
		report.DataGaps = append(report.DataGaps, "No deployment event found despite symptom detection")
	}
	if !hasMetric {
		report.DataGaps = append(report.DataGaps, "No metric events in causal chain — limited quantitative evidence")
	}
	if len(ctx.SimilarIncidents) == 0 {
		report.DataGaps = append(report.DataGaps, "No similar historical incidents found — novel failure pattern")
	}

	// Alternative hypotheses
	if len(ctx.SimilarIncidents) > 1 {
		end := 3
		if len(ctx.SimilarIncidents) < end {
			end = len(ctx.SimilarIncidents)
		}
		for _, si := range ctx.SimilarIncidents[1:end] {
			report.AlternativeHypotheses = append(report.AlternativeHypotheses, fmt.Sprintf("Similar to %s (similarity: %.2f, shape: %s)", si.IncidentID, si.Similarity, si.Shape))
		}
	}

	return report
}

func joinParts(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}

func (r *IncidentReconstructor) buildCausalChain(view *models.GraphView) []*models.CausalEvent {
	var events []*models.CausalEvent

	for _, node := range view.Nodes {
		service := ""
		kind := string(node.Type)

		if attrs, ok := node.Attributes["service"].(string); ok {
			service = attrs
		} else if attrs, ok := node.Attributes["service_name"].(string); ok {
			service = attrs
		} else {
			service = node.Label
		}

		description := ""
		if msg, ok := node.Attributes["message"].(string); ok {
			description = msg
		} else if msg, ok := node.Attributes["error"].(string); ok {
			description = msg
		}

		causalEvent := &models.CausalEvent{
			EventID:       node.ID,
			Timestamp:     node.CreatedAt,
			Kind:          kind,
			Service:       service,
			Description:   description,
			Attributes:    node.Attributes,
			SignalDensity: r.signalScore(node),
		}
		events = append(events, causalEvent)
	}

	return events
}

// signalScore computes a signal density score for an event node.
// Higher scores indicate more relevant/signal-dense events.
func (r *IncidentReconstructor) signalScore(node *models.Node) float64 {
	switch node.Type {
	case models.NodeTypeIncident:
		return 1.0
	case models.NodeTypeDeployment:
		return 0.6
	case models.NodeTypeMetric:
		// Anomaly metrics score higher
		if anomaly, ok := node.Attributes["anomaly"].(bool); ok && anomaly {
			return 0.9
		}
		if value, ok := node.Attributes["value"].(float64); ok {
			if threshold, ok := node.Attributes["threshold"].(float64); ok && value > threshold {
				return 0.85
			}
		}
		return 0.3
	case models.NodeTypeLog:
		if level, ok := node.Attributes["level"].(string); ok {
			switch level {
			case "error", "fatal":
				return 0.8
			case "warn":
				return 0.4
			}
		}
		if severity, ok := node.Attributes["severity"].(string); ok {
			if severity == "error" || severity == "critical" {
				return 0.8
			}
		}
		return 0.1
	case models.NodeTypeService:
		return 0.2
	default:
		return 0.1
	}
}

func (r *IncidentReconstructor) extractSymptoms(events []*models.CausalEvent) []models.Symptom {
	var symptoms []models.Symptom

	for _, event := range events {
		if event.Kind == "metric" || event.Kind == string(models.EventKindMetric) {
			anomaly := false
			if a, ok := event.Attributes["anomaly"].(bool); ok && a {
				anomaly = true
			} else if a, ok := event.Attributes["is_anomaly"].(bool); ok && a {
				anomaly = true
			}

			if !anomaly {
				continue
			}

			metricName := ""
			if m, ok := event.Attributes["metric_name"].(string); ok {
				metricName = m
			} else if m, ok := event.Attributes["metric"].(string); ok {
				metricName = m
			}

			var value float64
			if v, ok := event.Attributes["value"].(float64); ok {
				value = v
			} else if v, ok := event.Attributes["value"].(float32); ok {
				value = float64(v)
			}

			symptomType := models.SymptomTypeLatencySpike
			if metricName == "error" || metricName == "error_rate" {
				symptomType = models.SymptomTypeErrorRate
			} else if metricName == "timeout" {
				symptomType = models.SymptomTypeTimeout
			} else if metricName == "memory" {
				symptomType = models.SymptomTypeMemoryLeak
			} else if metricName == "cpu" {
				symptomType = models.SymptomTypeCPUSpike
			}

			symptoms = append(symptoms, models.Symptom{
				Type:        symptomType,
				Service:     event.Service,
				Metric:      metricName,
				Observed:    value,
				Description: fmt.Sprintf("%s spike detected for %s", symptomType, metricName),
			})
			continue
		}

		if event.Kind == "log" || event.Kind == string(models.EventKindLog) {
			level := ""
			if l, ok := event.Attributes["level"].(string); ok {
				level = l
			} else if l, ok := event.Attributes["severity"].(string); ok {
				level = l
			}

			if level != "error" && level != "ERROR" && level != "err" {
				continue
			}

			symptoms = append(symptoms, models.Symptom{
				Type:        models.SymptomTypeErrorRate,
				Service:     event.Service,
				Description: event.Description,
			})
			continue
		}

		if event.Kind == "deployment" || event.Kind == string(models.EventKindDeploy) {
			symptoms = append(symptoms, models.Symptom{
				Type:        models.SymptomTypeDeploymentFail,
				Service:     event.Service,
				Description: "Deployment occurred before incident",
			})
		}
	}

	return symptoms
}

func (r *IncidentReconstructor) computeConfidence(context *models.IncidentContext) float64 {
	var confidence float64 = 0.5
	cw := r.confidence

	if len(context.CausalChain) > 0 {
		confidence += cw.CausalChainBonus
		if len(context.CausalChain) > 5 {
			confidence += cw.CausalChainBonus
		}
	}

	if len(context.Symptoms) > 0 {
		confidence += cw.SymptomBonus
		if len(context.Symptoms) > 2 {
			confidence += cw.SymptomBonus
		}
	}

	if len(context.SimilarIncidents) > 0 {
		confidence += cw.SimilarityWeight * context.SimilarIncidents[0].Similarity
	}

	if len(context.SuggestedActions) > 0 {
		confidence += cw.RemediationWeight * context.SuggestedActions[0].SuccessRate
	}

	if confidence > cw.MaxConfidence {
		confidence = cw.MaxConfidence
	}
	if confidence < cw.MinConfidence {
		confidence = cw.MinConfidence
	}

	return confidence
}

// reconstructAdaptive performs adaptive context reconstruction with signal-density
// filtering and time-windowed traversal.
func (r *IncidentReconstructor) reconstructAdaptive(ctx context.Context, incidentID string, maxDepth, maxEvents int, timeWindow time.Duration) (*models.IncidentContext, error) {
	incident, err := r.incidentStore.Get(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("incident %s not found: %w", incidentID, err)
	}

	var incidentTime time.Time
	if incident != nil {
		incidentTime = incident.Timestamp
	} else {
		incidentTime = time.Now()
	}

	// Skip time window filtering if incident has zero timestamp (e.g., in tests)
	var windowMin, windowMax *time.Time
	if !incidentTime.IsZero() {
		min := incidentTime.Add(-timeWindow)
		max := incidentTime.Add(timeWindow)
		windowMin = &min
		windowMax = &max
	}

	graphView, err := r.graphBuilder.TraverseGraph(ctx, incidentID, models.TraversalOptions{
		MaxDepth:      maxDepth,
		EdgeTypes:    r.getReconstructionEdgeTypes(),
		TimeWindowMin: windowMin,
		TimeWindowMax: windowMax,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to traverse graph: %w", err)
	}

	result := &models.IncidentContext{
		IncidentID: incidentID,
	}
	if incident != nil {
		result.Timestamp = incident.Timestamp
	}

	causalChain := r.buildCausalChain(graphView)

	// Sort by signal density (highest first), then by time
	sort.Slice(causalChain, func(i, j int) bool {
		if causalChain[i].SignalDensity != causalChain[j].SignalDensity {
			return causalChain[i].SignalDensity > causalChain[j].SignalDensity
		}
		return causalChain[i].Timestamp.Before(causalChain[j].Timestamp)
	})

	// Limit to top N events by signal density
	if len(causalChain) > maxEvents {
		causalChain = causalChain[:maxEvents]
	}

	// Re-sort by time for the final causal chain
	sort.Slice(causalChain, func(i, j int) bool {
		return causalChain[i].Timestamp.Before(causalChain[j].Timestamp)
	})

	result.CausalChain = causalChain

	relatedServices := make(map[string]bool)
	for _, event := range causalChain {
		if event.Service != "" {
			relatedServices[event.Service] = true
		}
	}
	for service := range relatedServices {
		result.RelatedServices = append(result.RelatedServices, service)
	}

	result.Symptoms = r.extractSymptoms(causalChain)

	sig, err := r.matcher.ExtractSignature(ctx, result)
	if err == nil {
		similar, simErr := r.matcher.FindSimilar(ctx, sig, 5)
		if simErr == nil {
			result.SimilarIncidents = similar
		}

		if len(similar) > 0 {
			stats, actionErr := r.remediation.GetTopRemediations(ctx, similar[0].IncidentID, 5)
			if actionErr == nil {
				for _, s := range stats {
					result.SuggestedActions = append(result.SuggestedActions, models.RemediationAction{
						Action:           s.Action,
						Description:      fmt.Sprintf("Historical success rate: %.0f%%", s.SuccessRate*100),
						SuccessRate:      s.SuccessRate,
						AvgTimeToResolve: s.AvgTimeToResolve,
						Priority:         1,
					})
				}
			}
		}
	}

	result.Confidence = r.computeConfidence(result)
	report := r.generateExplainReport(result)
	result.Explain = report.Narrative
	result.ExplainReport = report

	return result, nil
}

func (r *IncidentReconstructor) getReconstructionEdgeTypes() []models.EdgeType {
	return []models.EdgeType{
		models.EdgeTypeSameIncident,
		models.EdgeTypeCausedProbably,
		models.EdgeTypeDeployedBefore,
		models.EdgeTypeMetricSpikedAfter,
		models.EdgeTypeLogErrorAfter,
		models.EdgeTypeCoOccurred,
		models.EdgeTypeDeploymentAdjacent,
		models.EdgeTypeInferredCausality,
	}
}
