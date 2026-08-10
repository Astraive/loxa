package grpcserver

import (
	"github.com/astraive/loza/cortex/internal/api"
	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/redaction"
	"github.com/astraive/loza/cortex/internal/storage"
)

func New(cfg *config.Config, stor storage.Storage) *api.GRPCServer {
	redactCfg := redaction.Config{
		Mode:      redaction.Mode(cfg.PIIRedaction.Mode),
		Blocklist: cfg.PIIRedaction.Blocklist,
		Allowlist: cfg.PIIRedaction.Allowlist,
	}
	if cfg.PIIRedaction.Enabled && redactCfg.Mode == "" && len(redactCfg.Blocklist) == 0 {
		redactCfg.Mode = redaction.ModeEnforce
	}
	if len(redactCfg.Allowlist) == 0 && cfg.PIIRedaction.Enabled {
		// Default allowlist for common non-sensitive event fields
		redactCfg.Allowlist = []string{"service", "level", "timestamp", "id", "event_id", "trace_id", "span_id", "incident_id", "environment", "release", "version", "schema_version", "event_version"}
	}
	return api.NewGRPCServer(cfg, stor, redactCfg)
}
