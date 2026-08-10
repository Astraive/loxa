package slog

import (
	"context"
	"log/slog"

	"github.com/astraive/loza/sdks/go"
)

// SlogHandler is a slog.Handler that forwards records to loza.
type SlogHandler struct {
	attrs  []slog.Attr
	groups []string
}

// Handler creates a slog handler backed by the default loza logger.
func Handler() *SlogHandler {
	return &SlogHandler{}
}

// Deprecated: use Handler.
func NewHandler() *SlogHandler { return Handler() }

func (h *SlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return mapLevel(level) >= loza.LevelDebug
}

func (h *SlogHandler) Handle(ctx context.Context, rec slog.Record) error {
	attrs := make([]loza.Attr, 0, rec.NumAttrs()+len(h.attrs))
	for _, a := range h.attrs {
		attrs = append(attrs, h.toAttr(a))
	}
	rec.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, h.toAttr(a))
		return true
	})
	attrs = wrapAttrsWithGroups(attrs, h.groups)

	if ev, ok := loza.FromContext(ctx); ok && ev != nil {
		loza.Enrich(ctx, attrs...)
		return nil
	}

	level := mapLevel(rec.Level)
	switch level {
	case loza.LevelDebug:
		loza.DebugContext(ctx, rec.Message, "slog.event", attrs...)
	case loza.LevelInfo:
		loza.InfoContext(ctx, rec.Message, "slog.event", attrs...)
	case loza.LevelWarn:
		loza.WarnContext(ctx, rec.Message, "slog.event", attrs...)
	default:
		loza.ErrorContext(ctx, rec.Message, nil, "slog.event", attrs...)
	}
	return nil
}

func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := *h
	out.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &out
}

func (h *SlogHandler) WithGroup(name string) slog.Handler {
	out := *h
	out.groups = append(append([]string(nil), h.groups...), name)
	return &out
}

func (h *SlogHandler) toAttr(a slog.Attr) loza.Attr {
	key := a.Key
	v := a.Value
	switch v.Kind() {
	case slog.KindGroup:
		children := v.Group()
		nested := make([]loza.Attr, 0, len(children))
		for _, child := range children {
			nested = append(nested, h.toAttr(child))
		}
		return loza.Group(key, nested...)
	case slog.KindString:
		return loza.String(key, v.String())
	case slog.KindInt64:
		return loza.Int64(key, v.Int64())
	case slog.KindUint64:
		return loza.Uint64(key, v.Uint64())
	case slog.KindFloat64:
		return loza.Float64(key, v.Float64())
	case slog.KindBool:
		return loza.Bool(key, v.Bool())
	case slog.KindDuration:
		return loza.Duration(key, v.Duration())
	case slog.KindTime:
		return loza.Time(key, v.Time())
	default:
		return loza.Any(key, v.Any())
	}
}

func wrapAttrsWithGroups(attrs []loza.Attr, groups []string) []loza.Attr {
	if len(groups) == 0 || len(attrs) == 0 {
		return attrs
	}
	out := attrs
	for i := len(groups) - 1; i >= 0; i-- {
		out = []loza.Attr{loza.Group(groups[i], out...)}
	}
	return out
}

func mapLevel(level slog.Level) loza.Level {
	switch {
	case level <= slog.LevelDebug:
		return loza.LevelDebug
	case level < slog.LevelWarn:
		return loza.LevelInfo
	case level < slog.LevelError:
		return loza.LevelWarn
	default:
		return loza.LevelError
	}
}
