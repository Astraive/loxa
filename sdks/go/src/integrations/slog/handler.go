package slog

import (
	"context"
	"log/slog"

	"github.com/astraive/loxa-go"
)

// SlogHandler is a slog.Handler that forwards records to loxa.
type SlogHandler struct {
	attrs  []slog.Attr
	groups []string
}

// Handler creates a slog handler backed by the default loxa logger.
func Handler() *SlogHandler {
	return &SlogHandler{}
}

// Deprecated: use Handler.
func NewHandler() *SlogHandler { return Handler() }

func (h *SlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return mapLevel(level) >= loxa.LevelDebug
}

func (h *SlogHandler) Handle(ctx context.Context, rec slog.Record) error {
	attrs := make([]loxa.Attr, 0, rec.NumAttrs()+len(h.attrs))
	for _, a := range h.attrs {
		attrs = append(attrs, h.toAttr(a))
	}
	rec.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, h.toAttr(a))
		return true
	})
	attrs = wrapAttrsWithGroups(attrs, h.groups)

	if ev, ok := loxa.FromContext(ctx); ok && ev != nil {
		loxa.Enrich(ctx, attrs...)
		return nil
	}

	level := mapLevel(rec.Level)
	switch level {
	case loxa.LevelDebug:
		loxa.DebugContext(ctx, rec.Message, "slog.event", attrs...)
	case loxa.LevelInfo:
		loxa.InfoContext(ctx, rec.Message, "slog.event", attrs...)
	case loxa.LevelWarn:
		loxa.WarnContext(ctx, rec.Message, "slog.event", attrs...)
	default:
		loxa.ErrorContext(ctx, rec.Message, nil, "slog.event", attrs...)
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

func (h *SlogHandler) toAttr(a slog.Attr) loxa.Attr {
	key := a.Key
	v := a.Value
	switch v.Kind() {
	case slog.KindGroup:
		children := v.Group()
		nested := make([]loxa.Attr, 0, len(children))
		for _, child := range children {
			nested = append(nested, h.toAttr(child))
		}
		return loxa.Group(key, nested...)
	case slog.KindString:
		return loxa.String(key, v.String())
	case slog.KindInt64:
		return loxa.Int64(key, v.Int64())
	case slog.KindUint64:
		return loxa.Uint64(key, v.Uint64())
	case slog.KindFloat64:
		return loxa.Float64(key, v.Float64())
	case slog.KindBool:
		return loxa.Bool(key, v.Bool())
	case slog.KindDuration:
		return loxa.Duration(key, v.Duration())
	case slog.KindTime:
		return loxa.Time(key, v.Time())
	default:
		return loxa.Any(key, v.Any())
	}
}

func wrapAttrsWithGroups(attrs []loxa.Attr, groups []string) []loxa.Attr {
	if len(groups) == 0 || len(attrs) == 0 {
		return attrs
	}
	out := attrs
	for i := len(groups) - 1; i >= 0; i-- {
		out = []loxa.Attr{loxa.Group(groups[i], out...)}
	}
	return out
}

func mapLevel(level slog.Level) loxa.Level {
	switch {
	case level <= slog.LevelDebug:
		return loxa.LevelDebug
	case level < slog.LevelWarn:
		return loxa.LevelInfo
	case level < slog.LevelError:
		return loxa.LevelWarn
	default:
		return loxa.LevelError
	}
}
