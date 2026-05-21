package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	transportcontracts "github.com/astraive/loxa-spec/transport/contracts"
	"github.com/astraive/loxa/loxa-cortex/internal/config"
	"github.com/astraive/loxa/loxa-cortex/internal/graph"
	"github.com/astraive/loxa/loxa-cortex/internal/learner"
	"github.com/astraive/loxa/loxa-cortex/internal/matcher"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
	"github.com/astraive/loxa/loxa-cortex/internal/processor"
	"github.com/astraive/loxa/loxa-cortex/internal/reconstructor"
	"github.com/astraive/loxa/loxa-cortex/internal/storage"
	"github.com/astraive/loxa/loxa-cortex/internal/topology"
)

type GraphQLServer struct {
	config      *config.Config
	processor   *processor.EventProcessor
	topology    *topology.Resolver
	graph       *graph.Builder
	match       matcher.SignatureService
	remediation *learner.Learner
	recon       *reconstructor.IncidentReconstructor
	incidents   storage.IncidentStore
	signatures  storage.SignatureStore
}

func NewGraphQLServer(cfg *config.Config, stor storage.Storage) *GraphQLServer {
	topology := topology.NewResolver(stor.Topology())
	graphBuilder := graph.NewBuilder(stor.Graph())
	matching, err := matcher.NewConfiguredSignatureMatcher(stor.Signatures(), cfg.Matcher)
	if err != nil {
		matching = matcher.NewSignatureMatcher(stor.Signatures())
	}
	remediation := learner.NewLearner(stor.Remediations(), stor.Feedback(), stor.Signatures())
	recon := reconstructor.NewIncidentReconstructor(graphBuilder, matching, remediation, stor.Incidents())
	eventProc := processor.NewEventProcessor(stor.Events(), stor.Topology(), stor.Graph())

	return &GraphQLServer{
		config:      cfg,
		processor:   eventProc,
		topology:    topology,
		graph:       graphBuilder,
		match:       matching,
		remediation: remediation,
		recon:       recon,
		incidents:   stor.Incidents(),
		signatures:  stor.Signatures(),
	}
}

func (s *GraphQLServer) Handler() http.Handler {
	return http.HandlerFunc(s.handleGraphQL)
}

func (s *GraphQLServer) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req transportcontracts.GraphQLRequest
	if r.Method == "GET" {
		req.Query = r.URL.Query().Get("query")
	} else {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "Failed to read request body")
			return
		}
		defer r.Body.Close()

		if err := json.Unmarshal(body, &req); err != nil {
			s.writeError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}
	}

	ctx := r.Context()
	result, err := s.executeQuery(ctx, req.Query, req.Variables)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := transportcontracts.GraphQLResponse{Data: result}
	json.NewEncoder(w).Encode(response)
}

func (s *GraphQLServer) writeError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	response := transportcontracts.GraphQLResponse{
		Errors: []transportcontracts.GraphQLError{{Message: message}},
	}
	json.NewEncoder(w).Encode(response)
}

func (s *GraphQLServer) executeQuery(ctx context.Context, query string, vars map[string]interface{}) (interface{}, error) {
	query = removeWhitespace(query)

	switch {
	case containsOperation(query, "ingestEvent"):
		return s.handleIngestEvent(ctx, vars)
	case containsOperation(query, "ingestBatch"):
		return s.handleIngestBatch(ctx, vars)
	case containsOperation(query, "reconstruct"):
		return s.handleReconstruct(ctx, vars)
	case containsOperation(query, "incident"):
		return s.handleIncident(ctx, vars)
	case containsOperation(query, "serviceGraph"):
		return s.handleServiceGraph(ctx, vars)
	case containsOperation(query, "incidentGraph"):
		return s.handleIncidentGraph(ctx, vars)
	case containsOperation(query, "remediationSuggestions"):
		return s.handleRemediationSuggestions(ctx, vars)
	case containsOperation(query, "similarIncidents"):
		return s.handleSimilarIncidents(ctx, vars)
	case containsOperation(query, "signature"):
		return s.handleSignature(ctx, vars)
	case containsOperation(query, "signatures"):
		return s.handleSignatures(ctx, vars)
	default:
		return nil, fmt.Errorf("unknown operation")
	}
}

func containsOperation(query, op string) bool {
	return len(query) > len(op) && (query[:len(op)] == op[:len(op)] || len(query) > len(op)+10 && containsWord(query, op))
}

func containsWord(s, word string) bool {
	for i := 0; i <= len(s)-len(word); i++ {
		if i > 0 && isAlpha(rune(s[i-1])) {
			continue
		}
		if i+len(word) <= len(s) && !isAlpha(rune(s[i+len(word)])) {
			if s[i:i+len(word)] == word {
				return true
			}
		}
	}
	return false
}

func isAlpha(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_'
}

func removeWhitespace(s string) string {
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			continue
		}
		result = append(result, r)
	}
	return string(result)
}

func (s *GraphQLServer) handleIngestEvent(ctx context.Context, vars map[string]interface{}) (interface{}, error) {
	eventMap, ok := vars["event"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("event variables required")
	}

	event := &models.Event{}
	if id, ok := eventMap["id"].(string); ok {
		event.ID = id
	}
	if service, ok := eventMap["service"].(string); ok {
		event.Service = service
	}
	if kind, ok := eventMap["kind"].(string); ok {
		event.Kind = models.EventKind(kind)
	}
	if provenance, ok := eventMap["provenance"].(string); ok {
		event.Provenance = provenance
	}

	event.Raw = make(map[string]interface{})
	for k, v := range eventMap {
		if k != "id" && k != "service" && k != "kind" && k != "provenance" {
			event.Raw[k] = v
		}
	}

	if err := s.processor.ProcessEvent(ctx, event); err != nil {
		return nil, err
	}

	return map[string]interface{}{"status": "accepted"}, nil
}

func (s *GraphQLServer) handleIngestBatch(ctx context.Context, vars map[string]interface{}) (interface{}, error) {
	eventsRaw, ok := vars["events"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("events array required")
	}

	events := make([]*models.Event, 0, len(eventsRaw))
	for _, e := range eventsRaw {
		eventMap, ok := e.(map[string]interface{})
		if !ok {
			continue
		}

		event := &models.Event{}
		if id, ok := eventMap["id"].(string); ok {
			event.ID = id
		}
		if service, ok := eventMap["service"].(string); ok {
			event.Service = service
		}
		if kind, ok := eventMap["kind"].(string); ok {
			event.Kind = models.EventKind(kind)
		}
		if provenance, ok := eventMap["provenance"].(string); ok {
			event.Provenance = provenance
		}

		event.Raw = make(map[string]interface{})
		for k, v := range eventMap {
			if k != "id" && k != "service" && k != "kind" && k != "provenance" {
				event.Raw[k] = v
			}
		}
		events = append(events, event)
	}

	if err := s.processor.ProcessBatch(ctx, events); err != nil {
		return nil, err
	}

	return map[string]interface{}{"status": "accepted", "count": len(events)}, nil
}

func (s *GraphQLServer) handleReconstruct(ctx context.Context, vars map[string]interface{}) (interface{}, error) {
	incidentID, ok := vars["incidentId"].(string)
	if !ok {
		return nil, fmt.Errorf("incidentId required")
	}

	mode := "fast"
	if m, ok := vars["mode"].(string); ok {
		mode = m
	}

	var recon *models.IncidentContext
	var err error
	if mode == "deep" {
		recon, err = s.recon.ReconstructDeep(ctx, incidentID)
	} else {
		recon, err = s.recon.ReconstructFast(ctx, incidentID)
	}

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"incidentId":       recon.IncidentID,
		"timestamp":        recon.Timestamp,
		"causalChain":      recon.CausalChain,
		"relatedServices":  recon.RelatedServices,
		"symptoms":         recon.Symptoms,
		"suggestedActions": recon.SuggestedActions,
		"confidence":       recon.Confidence,
	}, nil
}

func (s *GraphQLServer) handleIncident(ctx context.Context, vars map[string]interface{}) (interface{}, error) {
	incidentID, ok := vars["id"].(string)
	if !ok {
		return nil, fmt.Errorf("id required")
	}

	incident, err := s.incidents.Get(ctx, incidentID)
	if err != nil {
		return nil, err
	}

	return incident, nil
}

func (s *GraphQLServer) handleServiceGraph(ctx context.Context, vars map[string]interface{}) (interface{}, error) {
	service, ok := vars["service"].(string)
	if !ok {
		return nil, fmt.Errorf("service required")
	}

	depth := 3
	if d, ok := vars["depth"].(float64); ok {
		depth = int(d)
	}

	graphView, err := s.graph.GetServiceGraph(ctx, service, depth)
	if err != nil {
		return nil, err
	}

	return graphView, nil
}

func (s *GraphQLServer) handleIncidentGraph(ctx context.Context, vars map[string]interface{}) (interface{}, error) {
	incidentID, ok := vars["incidentId"].(string)
	if !ok {
		return nil, fmt.Errorf("incidentId required")
	}

	depth := 3
	if d, ok := vars["depth"].(float64); ok {
		depth = int(d)
	}

	graphView, err := s.graph.GetIncidentGraph(ctx, incidentID, depth)
	if err != nil {
		return nil, err
	}

	return graphView, nil
}

func (s *GraphQLServer) handleRemediationSuggestions(ctx context.Context, vars map[string]interface{}) (interface{}, error) {
	incidentID, ok := vars["incidentId"].(string)
	if !ok {
		return nil, fmt.Errorf("incidentId required")
	}

	_, err := s.incidents.Get(ctx, incidentID)
	if err != nil {
		return nil, err
	}

	sigs, err := s.signatures.List(ctx, 10)
	if err != nil {
		return nil, err
	}

	suggestions := make([]map[string]interface{}, 0)
	for _, sig := range sigs {
		similar, err := s.match.FindSimilar(ctx, sig, 5)
		if err != nil {
			continue
		}
		symptomType := ""
		if len(sig.Symptoms) > 0 {
			symptomType = string(sig.Symptoms[0])
		}
		for _, sim := range similar {
			suggestions = append(suggestions, map[string]interface{}{
				"action":     symptomType,
				"confidence": sim.Similarity,
				"signature":  sig,
			})
		}
	}

	return suggestions, nil
}

func (s *GraphQLServer) handleSimilarIncidents(ctx context.Context, vars map[string]interface{}) (interface{}, error) {
	incidentID, ok := vars["incidentId"].(string)
	if !ok {
		return nil, fmt.Errorf("incidentId required")
	}

	_, err := s.incidents.Get(ctx, incidentID)
	if err != nil {
		return nil, err
	}

	sigs, err := s.signatures.List(ctx, 10)
	if err != nil {
		return nil, err
	}

	similar := make([]*models.Incident, 0)
	for _, sig := range sigs {
		inc, err := s.incidents.GetBySignature(ctx, sig.SignatureID)
		if err == nil && inc != nil && inc.ID != incidentID {
			similar = append(similar, inc)
		}
	}

	return similar, nil
}

func (s *GraphQLServer) handleSignature(ctx context.Context, vars map[string]interface{}) (interface{}, error) {
	sigID, ok := vars["incidentId"].(string)
	if !ok {
		return nil, fmt.Errorf("incidentId required")
	}

	signature, err := s.signatures.Get(ctx, sigID)
	if err != nil {
		return nil, err
	}

	return signature, nil
}

func (s *GraphQLServer) handleSignatures(ctx context.Context, vars map[string]interface{}) (interface{}, error) {
	sigs, err := s.signatures.List(ctx, 50)
	if err != nil {
		return nil, err
	}

	return sigs, nil
}

var schema = `
type Event {
  id: ID!
  timestamp: String!
  kind: String!
  service: String!
  trace_id: String
  incident_id: String
  provenance: String!
}

type Incident {
  id: ID!
  timestamp: String!
  signature_id: String
  status: String!
  severity: String!
  primary_service: String!
  affected_services: [String!]!
  resolved_at: String
}

type Node {
  id: ID!
  type: String!
  label: String!
}

type Edge {
  id: ID!
  from_node_id: ID!
  to_node_id: ID!
  type: String!
  weight: Float!
}

type GraphView {
  nodes: [Node!]!
  edges: [Edge!]!
}

type RemediationAction {
  action: String!
  description: String!
  success_rate: Float!
  avg_time_to_resolve_seconds: Int!
}

type IncidentContext {
  incident_id: ID!
  timestamp: String!
  causal_chain: [CausalEvent!]!
  related_services: [String!]!
  symptoms: [Symptom!]!
  suggested_actions: [RemediationAction!]!
  confidence: Float!
}

type Signature {
  id: ID!
  hash: String!
  symptom_type: String!
  affected_services: [String!]!
  occurrence_count: Int!
  avg_resolution_time_seconds: Int64!
  created_at: String!
}

type Query {
  ingestEvent(event: EventInput!): ActionResult!
  ingestBatch(events: [EventInput!]!): BatchResult!
  reconstruct(incidentId: ID!, mode: String): IncidentContext!
  incident(id: ID!): Incident
  serviceGraph(service: String!, depth: Int): GraphView!
  incidentGraph(incidentId: ID!, depth: Int): GraphView!
  remediationSuggestions(incidentId: ID!): [Suggestion!]!
  similarIncidents(incidentId: ID!): [Incident!]!
  signature(id: ID!): Signature
  signatures: [Signature!]!
}

input EventInput {
  id: ID!
  service: String!
  kind: String!
  provenance: String
}

type ActionResult {
  status: String!
}

type BatchResult {
  status: String!
  count: Int!
}

type Suggestion {
  action: String!
  confidence: Float!
  signature: Signature!
}
`

func (s *GraphQLServer) Schema() string {
	return schema
}
