package core

// SanitizeEvent clones the event and applies the global config's redactor
// and security settings. The original event is never mutated.
func SanitizeEvent(ev *Event) *Event {
	if ev == nil {
		return nil
	}
	clone := ev.Clone()
	if clone == nil {
		return nil
	}
	clone.logger = nil
	cfg := Default().cfg
	if cfg.Redactor != nil {
		clone.Attrs = applyRedactor(clone.Attrs, cfg.Redactor)
	}
	if cfg.Security.MaxAttrCount > 0 || cfg.Security.MaxFieldBytes > 0 || (cfg.Security.RedactByDefault && !cfg.Security.AllowPII) {
		clone.Attrs = applySecurity(clone.Attrs, cfg.Security)
	}
	return clone
}
