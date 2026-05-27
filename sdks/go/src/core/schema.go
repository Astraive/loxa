package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// EventView is a read-only view over an event used by schemas.
type EventView interface {
	SchemaVersion() string
	EventVersion() string
	ID() string
	Name() string
	Kind() string
	Message() string
	RequestID() string
	TraceID() string
	SpanID() string
	ParentID() string
	IncidentID() string
	Timestamp() time.Time
	StartedAt() time.Time
	FinishedAt() time.Time
	DurationMS() int64
	Service() string
	Version() string
	Environment() string
	DeploymentID() string
	Region() string
	Host() string
	Runtime() string
	Method() string
	Path() string
	Route() string
	StatusCode() int
	Level() Level
	Outcome() string
	Error() *ErrorInfo

	Attr(key string) any
	Attrs() map[string]any
	Group(name string) map[string]any
	Checkpoints() []EventCheckpoint
	Processes() []EventProcess
	Groups() []EventGroup
	Timers() []EventTimer
}

// Schema controls final output shape for emitted events.
type Schema interface {
	Encode(event EventView) ([]byte, error)
}

// SchemaFunc allows mapping an EventView to an output object.
type SchemaFunc func(event EventView) map[string]any

// Encode converts map output to NDJSON.
func (f SchemaFunc) Encode(event EventView) ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("loxa: schema func is nil")
	}
	out := f(event)
	if out == nil {
		out = map[string]any{}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	return b, nil
}

// CustomSchema creates a schema from a projection function.
func CustomSchema(fn func(EventView) map[string]any) Schema {
	return SchemaFunc(fn)
}

// DefaultSchema emits canonical LOXA output shape.
func DefaultSchema() Schema {
	return SchemaFunc(defaultSchemaMap)
}

// FlatSchema emits flattened attrs for analytics databases.
func FlatSchema() Schema {
	return SchemaFunc(flatSchemaMap)
}

// NestedSchema emits canonical fields with nested attrs/groups.
func NestedSchema() Schema {
	return SchemaFunc(defaultSchemaMap)
}

// OTelLogSchema emits an OpenTelemetry-flavored log shape.
func OTelLogSchema() Schema {
	return SchemaFunc(func(ev EventView) map[string]any {
		out := map[string]any{
			"timestamp":  ev.Timestamp().UTC().Format(time.RFC3339Nano),
			"severity":   ev.Level().String(),
			"body":       ev.Name(),
			"attributes": ev.Attrs(),
		}
		if rid := ev.RequestID(); rid != "" {
			out["request_id"] = rid
		}
		if svc := ev.Service(); svc != "" {
			out["service.name"] = svc
		}
		if err := ev.Error(); err != nil {
			out["error"] = map[string]any{
				"type":      err.Type,
				"message":   err.Message,
				"code":      err.Code,
				"retriable": err.Retriable,
			}
		}
		return out
	})
}

// ECSchema emits an Elastic Common Schema-inspired log shape.
func ECSchema() Schema {
	return SchemaFunc(func(ev EventView) map[string]any {
		out := map[string]any{
			"@timestamp": ev.Timestamp().UTC().Format(time.RFC3339Nano),
			"event": map[string]any{
				"id":       ev.ID(),
				"action":   ev.Name(),
				"outcome":  ev.Outcome(),
				"duration": ev.DurationMS() * int64(time.Millisecond),
			},
			"log": map[string]any{
				"level": ev.Level().String(),
			},
			"labels": ev.Attrs(),
		}
		if svc := ev.Service(); svc != "" {
			out["service"] = map[string]any{"name": svc}
		}
		if rid := ev.RequestID(); rid != "" {
			out["trace"] = map[string]any{"id": rid}
		}
		if err := ev.Error(); err != nil {
			out["error"] = map[string]any{
				"type":    err.Type,
				"message": err.Message,
				"code":    err.Code,
				"stack":   err.Stack,
			}
		}
		return out
	})
}

// DatadogSchema emits a Datadog-like JSON shape.
func DatadogSchema() Schema {
	return SchemaFunc(func(ev EventView) map[string]any {
		out := map[string]any{
			"timestamp": ev.Timestamp().UTC().UnixMilli(),
			"status":    ev.Level().String(),
			"message":   ev.Name(),
			"service":   ev.Service(),
			"ddtags":    attrsToTagString(ev.Attrs()),
			"fields":    ev.Attrs(),
		}
		if rid := ev.RequestID(); rid != "" {
			out["request_id"] = rid
		}
		if err := ev.Error(); err != nil {
			out["error"] = map[string]any{
				"type":    err.Type,
				"message": err.Message,
				"code":    err.Code,
			}
		}
		return out
	})
}

func newEventView(ev *Event) EventView {
	if ev == nil {
		return &readOnlyEventView{}
	}
	snap := ev.Clone()
	attrs := attrsToMap(snap.Attrs, true)
	return &readOnlyEventView{
		ev:          snap,
		attrs:       attrs,
		checkpoints: cloneCheckpoints(snap.Checkpoints),
		processes:   cloneProcesses(snap.Processes),
		groups:      cloneGroups(snap.Groups),
		timers:      cloneTimers(snap.Timers),
	}
}

type readOnlyEventView struct {
	ev          *Event
	attrs       map[string]any
	checkpoints []EventCheckpoint
	processes   []EventProcess
	groups      []EventGroup
	timers      []EventTimer
}

func (v *readOnlyEventView) ID() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.EventID
}

func (v *readOnlyEventView) SchemaVersion() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.SchemaVersion
}

func (v *readOnlyEventView) EventVersion() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.EventVersion
}

func (v *readOnlyEventView) Name() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Event
}

func (v *readOnlyEventView) Kind() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Kind
}

func (v *readOnlyEventView) Message() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Message
}

func (v *readOnlyEventView) RequestID() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.RequestID
}

func (v *readOnlyEventView) TraceID() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.TraceID
}

func (v *readOnlyEventView) SpanID() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.SpanID
}

func (v *readOnlyEventView) ParentID() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.ParentID
}

func (v *readOnlyEventView) IncidentID() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.IncidentID
}

func (v *readOnlyEventView) Timestamp() time.Time {
	if v.ev == nil {
		return time.Time{}
	}
	return v.ev.Timestamp
}

func (v *readOnlyEventView) StartedAt() time.Time {
	if v.ev == nil {
		return time.Time{}
	}
	return v.ev.StartedAt
}

func (v *readOnlyEventView) FinishedAt() time.Time {
	if v.ev == nil {
		return time.Time{}
	}
	return v.ev.FinishedAt
}

func (v *readOnlyEventView) DurationMS() int64 {
	if v.ev == nil {
		return 0
	}
	return v.ev.DurationMS
}

func (v *readOnlyEventView) Service() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Service
}

func (v *readOnlyEventView) Version() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Version
}

func (v *readOnlyEventView) Environment() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Environment
}

func (v *readOnlyEventView) DeploymentID() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.DeploymentID
}

func (v *readOnlyEventView) Region() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Region
}

func (v *readOnlyEventView) Host() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Host
}

func (v *readOnlyEventView) Runtime() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Runtime
}

func (v *readOnlyEventView) Method() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Method
}

func (v *readOnlyEventView) Path() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Path
}

func (v *readOnlyEventView) Route() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Route
}

func (v *readOnlyEventView) StatusCode() int {
	if v.ev == nil {
		return 0
	}
	return v.ev.StatusCode
}

func (v *readOnlyEventView) Level() Level {
	if v.ev == nil {
		return LevelInfo
	}
	return v.ev.Level
}

func (v *readOnlyEventView) Outcome() string {
	if v.ev == nil {
		return ""
	}
	return v.ev.Outcome
}

func (v *readOnlyEventView) Error() *ErrorInfo {
	if v.ev == nil || v.ev.Error == nil {
		return nil
	}
	cp := *v.ev.Error
	return &cp
}

func (v *readOnlyEventView) Attr(key string) any {
	out, _ := lookupPath(v.attrs, key)
	return out
}

func (v *readOnlyEventView) Attrs() map[string]any {
	return cloneAnyMap(v.attrs)
}

func (v *readOnlyEventView) Group(name string) map[string]any {
	raw, ok := lookupPath(v.attrs, name)
	if !ok {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return cloneAnyMap(m)
}

func (v *readOnlyEventView) Checkpoints() []EventCheckpoint {
	return cloneCheckpoints(v.checkpoints)
}

func (v *readOnlyEventView) Processes() []EventProcess {
	return cloneProcesses(v.processes)
}

func (v *readOnlyEventView) Groups() []EventGroup {
	return cloneGroups(v.groups)
}

func (v *readOnlyEventView) Timers() []EventTimer {
	return cloneTimers(v.timers)
}

func defaultSchemaMap(ev EventView) map[string]any {
	out := map[string]any{
		"timestamp":      ev.Timestamp().UTC().Format(time.RFC3339),
		"schema_version": firstNonEmpty(ev.SchemaVersion(), LOXA_SPEC_VERSION),
		"event_version":  firstNonEmpty(ev.EventVersion(), LOXA_EVENT_VERSION),
		"event_id":       ev.ID(),
		"request_id":     ev.RequestID(),
		"level":          ev.Level().String(),
		"event":          ev.Name(),
		"kind":           firstNonEmpty(ev.Kind(), "event"),
		"outcome":        ev.Outcome(),
		"duration_ms":    ev.DurationMS(),
	}
	if msg := ev.Message(); msg != "" {
		out["message"] = msg
	}
	if traceID := ev.TraceID(); traceID != "" {
		out["trace_id"] = traceID
	}
	if spanID := ev.SpanID(); spanID != "" {
		out["span_id"] = spanID
	}
	if parentID := ev.ParentID(); parentID != "" {
		out["parent_id"] = parentID
	}
	if incidentID := ev.IncidentID(); incidentID != "" {
		out["incident_id"] = incidentID
	}
	if svc := ev.Service(); svc != "" {
		out["service"] = svc
	}
	if version := ev.Version(); version != "" {
		out["version"] = version
	}
	if environment := ev.Environment(); environment != "" {
		out["environment"] = environment
	}
	if deploymentID := ev.DeploymentID(); deploymentID != "" {
		out["deployment_id"] = deploymentID
	}
	if region := ev.Region(); region != "" {
		out["region"] = region
	}
	if host := ev.Host(); host != "" {
		out["host"] = host
	}
	if runtime := ev.Runtime(); runtime != "" {
		out["runtime"] = runtime
	}
	if method := ev.Method(); method != "" {
		out["method"] = method
	}
	if path := ev.Path(); path != "" {
		out["path"] = path
	}
	if route := ev.Route(); route != "" {
		out["route"] = route
	}
	if statusCode := ev.StatusCode(); statusCode != 0 {
		out["status_code"] = statusCode
	}

	attrs := ev.Attrs()
	httpGroup, userGroup, tenantGroup, resourceGroup, extraAttrs := partitionStructuredAttrs(attrs)
	if ev.Method() != "" || ev.Path() != "" || ev.Route() != "" || ev.StatusCode() != 0 || len(httpGroup) > 0 {
		httpOut := mergeHTTPPayload(httpGroup, ev.Method(), ev.Path(), ev.Route(), ev.StatusCode())
		if len(httpOut) > 0 {
			out["http"] = httpOut
		}
	}
	if isNonEmptyMap(userGroup) {
		out["user"] = userGroup
	}
	if isNonEmptyMap(tenantGroup) {
		out["tenant"] = tenantGroup
	}
	if isNonEmptyMap(resourceGroup) {
		out["resource"] = resourceGroup
	}
	if isNonEmptyMap(extraAttrs) {
		out["attrs"] = extraAttrs
	}
	if err := ev.Error(); err != nil {
		out["error"] = map[string]any{
			"type":      err.Type,
			"message":   err.Message,
			"code":      err.Code,
			"retriable": err.Retriable,
			"cause":     err.Cause,
			"stack":     err.Stack,
		}
	}
	if cps := ev.Checkpoints(); len(cps) > 0 {
		items := make([]map[string]any, 0, len(cps))
		for _, cp := range cps {
			item := map[string]any{"name": cp.Name, "at_ms": cp.AtMS}
			group := attrsToMap(cp.Attrs, true)
			for k, v := range group {
				item[k] = v
			}
			items = append(items, item)
		}
		out["checkpoints"] = items
	}
	if procs := ev.Processes(); len(procs) > 0 {
		items := make([]map[string]any, 0, len(procs))
		for _, p := range procs {
			item := map[string]any{
				"step":          p.Step,
				"name":          p.Name,
				"started_at_ms": p.StartedAtMS,
				"ended_at_ms":   p.EndedAtMS,
				"duration_ms":   p.DurationMS,
			}
			if p.StatusCode != 0 {
				item["status_code"] = p.StatusCode
			}
			for k, v := range attrsToMap(p.Attrs, true) {
				item[k] = v
			}
			items = append(items, item)
		}
		out["processes"] = items
	}
	if grps := ev.Groups(); len(grps) > 0 {
		items := make([]map[string]any, 0, len(grps))
		for _, g := range grps {
			item := map[string]any{
				"name":          g.Name,
				"started_at_ms": g.StartedAtMS,
				"ended_at_ms":   g.EndedAtMS,
				"duration_ms":   g.DurationMS,
			}
			if g.StatusCode != 0 {
				item["status_code"] = g.StatusCode
			}
			for k, v := range attrsToMap(g.Attrs, true) {
				item[k] = v
			}
			items = append(items, item)
		}
		out["groups"] = items
	}
	if tmrs := ev.Timers(); len(tmrs) > 0 {
		items := make([]map[string]any, 0, len(tmrs))
		for _, t := range tmrs {
			item := map[string]any{
				"name":        t.Name,
				"duration_ms": t.DurationMS,
			}
			if t.StatusCode != 0 {
				item["status_code"] = t.StatusCode
			}
			for k, v := range attrsToMap(t.Attrs, true) {
				item[k] = v
			}
			items = append(items, item)
		}
		out["timers"] = items
	}
	return out
}

func flatSchemaMap(ev EventView) map[string]any {
	base := defaultSchemaMap(ev)
	attrs := ev.Attrs()
	flat := flattenAnyMap(attrs, "", "_")
	for k, v := range flat {
		base[k] = v
	}
	for k := range attrs {
		delete(base, k)
	}
	return base
}

func attrsToTagString(attrs map[string]any) string {
	flat := flattenAnyMap(attrs, "", ".")
	if len(flat) == 0 {
		return ""
	}
	keys := make([]string, 0, len(flat))
	for k := range flat {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%v", k, flat[k]))
	}
	return strings.Join(parts, ",")
}

func attrsToMap(attrs []Attr, expandDot bool) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]any, len(attrs))
	for _, a := range attrs {
		if a.Key == "" {
			continue
		}
		value := attrToAny(a)
		if a.Kind == KindGroup {
			children, _ := value.(map[string]any)
			if children == nil {
				children = map[string]any{}
			}
			existing, _ := out[a.Key].(map[string]any)
			out[a.Key] = mergeAnyMap(existing, children)
			continue
		}
		if expandDot && strings.Contains(a.Key, ".") {
			setPath(out, strings.Split(a.Key, "."), value)
			continue
		}
		out[a.Key] = value
	}
	return out
}

func attrToAny(a Attr) any {
	switch a.Kind {
	case KindGroup:
		children, _ := a.Value.([]Attr)
		return attrsToMap(children, true)
	case KindError:
		if err, ok := a.Value.(error); ok {
			return err.Error()
		}
	}
	return a.Value
}

func setPath(root map[string]any, parts []string, value any) {
	if len(parts) == 0 {
		return
	}
	cur := root
	for _, p := range parts[:len(parts)-1] {
		next, ok := cur[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
	cur[parts[len(parts)-1]] = value
}

func lookupPath(root map[string]any, path string) (any, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	var cur any = root
	for _, p := range parts {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := obj[p]
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAnyValue(v)
	}
	return out
}

func cloneAnyValue(v any) any {
	switch vv := v.(type) {
	case map[string]any:
		return cloneAnyMap(vv)
	case []any:
		out := make([]any, len(vv))
		for i := range vv {
			out[i] = cloneAnyValue(vv[i])
		}
		return out
	default:
		return vv
	}
}

func mergeAnyMap(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for k, v := range src {
		if dv, ok := dst[k].(map[string]any); ok {
			if sv, ok := v.(map[string]any); ok {
				dst[k] = mergeAnyMap(dv, sv)
				continue
			}
		}
		dst[k] = cloneAnyValue(v)
	}
	return dst
}

func flattenAnyMap(in map[string]any, prefix, sep string) map[string]any {
	out := make(map[string]any)
	for k, v := range in {
		key := k
		if prefix != "" {
			key = prefix + sep + k
		}
		if child, ok := v.(map[string]any); ok {
			for ck, cv := range flattenAnyMap(child, key, sep) {
				out[ck] = cv
			}
			continue
		}
		out[key] = v
	}
	return out
}
