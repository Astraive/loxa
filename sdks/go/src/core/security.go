package core

import "strings"

// SecurityConfig controls event-size and sensitive-data limits.
type SecurityConfig struct {
	RedactByDefault     bool
	AllowPII            bool
	MaxFieldBytes       int
	MaxEventBytes       int
	MaxAttrCount        int
	DropOversizedEvents bool
}

func applySecurity(attrs []Attr, cfg SecurityConfig) []Attr {
	if cfg.MaxAttrCount > 0 && len(attrs) > cfg.MaxAttrCount {
		attrs = append(attrs[:cfg.MaxAttrCount], Bool("_truncated", true))
	}
	if cfg.MaxFieldBytes <= 0 {
		return attrs
	}
	for i := range attrs {
		if attrs[i].Kind == KindGroup {
			if children, ok := attrs[i].Value.([]Attr); ok {
				attrs[i].Value = applySecurity(children, cfg)
			}
			continue
		}
		if s, ok := attrs[i].Value.(string); ok && len(s) > cfg.MaxFieldBytes {
			attrs[i].Value = s[:cfg.MaxFieldBytes]
		}
		if cfg.RedactByDefault && strings.HasPrefix(attrs[i].Key, "sensitive.") && !cfg.AllowPII {
			attrs[i].Value = "[REDACTED]"
			attrs[i].Kind = KindString
		}
	}
	return attrs
}
