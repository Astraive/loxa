package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/astraive/loxa/sdks/go/src/internal/jsonenc"
	"github.com/astraive/loxa/sdks/go/src/internal/pool"
)

// TimeFormat controls how timestamps are serialised.
type TimeFormat int

const (
	TimeFormatRFC3339     TimeFormat = iota // "2006-01-02T15:04:05Z07:00"
	TimeFormatRFC3339Nano                   // nanosecond precision
	TimeFormatUnixMS                        // milliseconds since epoch as integer
)

// Encoder converts an Event to its wire representation.
type Encoder interface {
	EncodeEvent(dst []byte, ev *Event) ([]byte, error)
}

// JSONEventEncoder is the default Encoder producing compact or pretty NDJSON.
type JSONEventEncoder struct {
	Pretty        bool
	ExpandDotKeys bool
	TimeFormat    TimeFormat
}

// EncodeEvent encodes ev into dst and returns the extended slice.
func (e *JSONEventEncoder) EncodeEvent(dst []byte, ev *Event) ([]byte, error) {
	ev.MuLock()
	defer ev.MuUnlock()

	w := jsonenc.New(pool.Get())
	w.BeginObject()

	// 1. timestamp
	ts := ev.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	w.AppendStringField("timestamp", e.formatTime(ts))
	w.AppendStringField("schema_version", firstNonEmpty(ev.SchemaVersion, LOXA_SPEC_VERSION))
	w.AppendStringField("event_version", firstNonEmpty(ev.EventVersion, LOXA_EVENT_VERSION))

	// 2. event_id
	if ev.EventID != "" {
		w.AppendStringField("event_id", ev.EventID)
	}
	// 3. request_id
	if ev.RequestID != "" {
		w.AppendStringField("request_id", ev.RequestID)
	}
	// 4-6. trace ids
	if ev.TraceID != "" {
		w.AppendStringField("trace_id", ev.TraceID)
	}
	if ev.SpanID != "" {
		w.AppendStringField("span_id", ev.SpanID)
	}
	if ev.ParentID != "" {
		w.AppendStringField("parent_id", ev.ParentID)
	}

	// 7. level
	w.AppendStringField("level", ev.Level.String())

	// 8-10. event / message / outcome
	if ev.Event != "" {
		w.AppendStringField("event", ev.Event)
	}
	if ev.Kind != "" {
		w.AppendStringField("kind", ev.Kind)
	}
	if ev.Message != "" {
		w.AppendStringField("message", ev.Message)
	}
	if ev.Outcome != "" {
		w.AppendStringField("outcome", ev.Outcome)
	}
	if ev.state != "" {
		state := ev.state
		if state == EventStateEmitting && ev.Outcome != "" {
			state = EventStateFinished
		}
		w.AppendStringField("event_state", string(state))
	}

	// 11. duration_ms
	if ev.DurationMS > 0 {
		w.AppendInt64Field("duration_ms", ev.DurationMS)
	}

	// 12. service metadata
	if ev.Service != "" {
		w.AppendStringField("service", ev.Service)
	}
	if ev.Version != "" {
		w.AppendStringField("version", ev.Version)
	}
	if ev.Environment != "" {
		w.AppendStringField("environment", ev.Environment)
	}
	if ev.DeploymentID != "" {
		w.AppendStringField("deployment_id", ev.DeploymentID)
	}
	if ev.Region != "" {
		w.AppendStringField("region", ev.Region)
	}
	if ev.Host != "" {
		w.AppendStringField("host", ev.Host)
	}
	if ev.Runtime != "" {
		w.AppendStringField("runtime", ev.Runtime)
	}

	// 13. request metadata
	if ev.Method != "" {
		w.AppendStringField("method", ev.Method)
	}
	if ev.Path != "" {
		w.AppendStringField("path", ev.Path)
	}
	if ev.Route != "" {
		w.AppendStringField("route", ev.Route)
	}
	if ev.StatusCode != 0 {
		w.AppendInt64Field("status_code", int64(ev.StatusCode))
	}

	attrs := attrsToMap(ev.Attrs, e.ExpandDotKeys)
	httpGroup, userGroup, tenantGroup, resourceGroup, extraAttrs := partitionStructuredAttrs(attrs)
	if httpPayload := mergeHTTPPayload(httpGroup, ev.Method, ev.Path, ev.Route, ev.StatusCode); len(httpPayload) > 0 {
		e.appendMapField(w, "http", httpPayload)
	}
	if isNonEmptyMap(userGroup) {
		e.appendMapField(w, "user", userGroup)
	}
	if isNonEmptyMap(tenantGroup) {
		e.appendMapField(w, "tenant", tenantGroup)
	}
	if isNonEmptyMap(resourceGroup) {
		e.appendMapField(w, "resource", resourceGroup)
	}
	if isNonEmptyMap(extraAttrs) {
		e.appendMapField(w, "attrs", extraAttrs)
	}

	// 15. error
	if ev.Error != nil {
		w.AppendKey("error")
		w.BeginObject()
		w.AppendStringField("type", ev.Error.Type)
		if ev.Error.Code != "" {
			w.AppendStringField("code", ev.Error.Code)
		}
		w.AppendStringField("message", ev.Error.Message)
		if ev.Error.Retriable {
			w.AppendBoolField("retriable", true)
		}
		if ev.Error.Cause != "" {
			w.AppendStringField("cause", ev.Error.Cause)
		}
		if ev.Error.Stack != "" {
			w.AppendStringField("stack", ev.Error.Stack)
		}
		w.EndObject()
	}

	// 16. checkpoints
	if len(ev.Checkpoints) > 0 {
		w.AppendKey("checkpoints")
		w.BeginArray()
		for i := range ev.Checkpoints {
			cp := &ev.Checkpoints[i]
			w.BeginObject()
			w.AppendStringField("name", cp.Name)
			w.AppendInt64Field("at_ms", cp.AtMS)
			e.appendMapEntries(w, attrsToMap(cp.Attrs, e.ExpandDotKeys))
			w.EndObject()
		}
		w.EndArray()
	}

	// 17. processes
	if len(ev.Processes) > 0 {
		w.AppendKey("processes")
		w.BeginArray()
		for i := range ev.Processes {
			p := &ev.Processes[i]
			w.BeginObject()
			w.AppendInt64Field("step", int64(p.Step))
			w.AppendStringField("name", p.Name)
			if p.StatusCode != 0 {
				w.AppendInt64Field("status_code", int64(p.StatusCode))
			}
			w.AppendInt64Field("started_at_ms", p.StartedAtMS)
			w.AppendInt64Field("ended_at_ms", p.EndedAtMS)
			w.AppendInt64Field("duration_ms", p.DurationMS)
			e.appendMapEntries(w, attrsToMap(p.Attrs, e.ExpandDotKeys))
			w.EndObject()
		}
		w.EndArray()
	}

	// 18. groups
	if len(ev.Groups) > 0 {
		w.AppendKey("groups")
		w.BeginArray()
		for i := range ev.Groups {
			g := &ev.Groups[i]
			w.BeginObject()
			w.AppendStringField("name", g.Name)
			if g.StatusCode != 0 {
				w.AppendInt64Field("status_code", int64(g.StatusCode))
			}
			w.AppendInt64Field("started_at_ms", g.StartedAtMS)
			w.AppendInt64Field("ended_at_ms", g.EndedAtMS)
			w.AppendInt64Field("duration_ms", g.DurationMS)
			e.appendMapEntries(w, attrsToMap(g.Attrs, e.ExpandDotKeys))
			w.EndObject()
		}
		w.EndArray()
	}

	// 19. timers
	if len(ev.Timers) > 0 {
		w.AppendKey("timers")
		w.BeginArray()
		for i := range ev.Timers {
			t := &ev.Timers[i]
			w.BeginObject()
			w.AppendStringField("name", t.Name)
			w.AppendInt64Field("duration_ms", t.DurationMS)
			if t.StatusCode != 0 {
				w.AppendInt64Field("status_code", int64(t.StatusCode))
			}
			e.appendMapEntries(w, attrsToMap(t.Attrs, e.ExpandDotKeys))
			w.EndObject()
		}
		w.EndArray()
	}

	w.EndObject()

	raw := w.Bytes()
	if e.Pretty {
		dst = prettyAppend(dst, raw)
	} else {
		dst = append(dst, raw...)
	}
	dst = append(dst, '\n')
	pool.Put(raw)
	return dst, nil
}

func (e *JSONEventEncoder) appendMapField(w *jsonenc.Writer, key string, value map[string]any) {
	raw, err := json.Marshal(value)
	if err != nil {
		w.AppendStringField(key, fmt.Sprintf("<marshal error: %v>", err))
		return
	}
	w.AppendKey(key)
	w.AppendRaw(raw)
}

func (e *JSONEventEncoder) appendMapEntries(w *jsonenc.Writer, value map[string]any) {
	for key, field := range value {
		raw, err := json.Marshal(field)
		if err != nil {
			w.AppendStringField(key, fmt.Sprintf("<marshal error: %v>", err))
			continue
		}
		w.AppendKey(key)
		w.AppendRaw(raw)
	}
}

// JSONEncoder returns the default compact JSON encoder.
func JSONEncoder() *JSONEventEncoder {
	return &JSONEventEncoder{ExpandDotKeys: true, TimeFormat: TimeFormatRFC3339}
}

// PrettyJSONEncoder returns a pretty-print JSON encoder.
func PrettyJSONEncoder() *JSONEventEncoder {
	return &JSONEventEncoder{Pretty: true, ExpandDotKeys: true, TimeFormat: TimeFormatRFC3339}
}

func (e *JSONEventEncoder) formatTime(t time.Time) string {
	switch e.TimeFormat {
	case TimeFormatRFC3339Nano:
		return t.UTC().Format(time.RFC3339Nano)
	case TimeFormatUnixMS:
		return fmt.Sprintf("%d", t.UnixMilli())
	default:
		return t.UTC().Format(time.RFC3339)
	}
}

func prettyAppend(dst, src []byte) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, src, "", "  "); err != nil {
		return append(dst, src...)
	}
	return append(dst, out.Bytes()...)
}
