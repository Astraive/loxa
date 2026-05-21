package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logJSON("error", "response_encode_failed", map[string]any{"error": err.Error()})
	}
}

func logJSON(level, message string, fields map[string]any) {
	body := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"level":     level,
		"message":   message,
	}
	for k, v := range fields {
		body[k] = v
	}
	b, err := json.Marshal(body)
	if err != nil {
		log.Printf(`{"level":"error","message":"collector_log_encode_failed","error":"%s"}`, err.Error())
		return
	}
	log.Print(string(b))
}

func boolMetric(v bool) int {
	if v {
		return 1
	}
	return 0
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
