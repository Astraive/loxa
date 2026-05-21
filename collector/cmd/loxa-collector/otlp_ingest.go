package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/astraive/loxa-collector/internal/otlpconv"
	collectorlogsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (s *collectorState) handleOTLPLogs(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_failed"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, s.cfg.maxBodyBytes))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_otlp_request", "message": err.Error()})
		return
	}

	req := &collectorlogsv1.ExportLogsServiceRequest{}
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	switch {
	case strings.Contains(contentType, "application/x-protobuf"), strings.Contains(contentType, "application/protobuf"):
		if err := proto.Unmarshal(body, req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_otlp_protobuf", "message": err.Error()})
			return
		}
	default:
		if err := protojson.Unmarshal(body, req); err != nil {
			if err := json.Unmarshal(body, req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_otlp_json", "message": err.Error()})
				return
			}
		}
	}

	events, err := otlpconv.ConvertExportLogsRequest(req, otlpconv.Config{
		SchemaVersion:  s.cfg.schemaSchemaVersion,
		EventVersion:   s.cfg.schemaEventVersion,
		DefaultService: s.cfg.boundServiceName,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "otlp_conversion_failed", "message": err.Error()})
		return
	}
	accepted, err := s.handleIngestBatch(r.Context(), events)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "otlp_ingest_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"partialSuccess": map[string]any{}, "accepted": accepted})
}
