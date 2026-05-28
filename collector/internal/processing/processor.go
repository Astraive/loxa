package processing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	collectorevent "github.com/astraive/loxa/collector/internal/event"
	"github.com/astraive/loxa/collector/internal/validation"
	"github.com/redis/go-redis/v9"
)

var ErrInvalidEvent = errors.New("invalid event payload: expected JSON object")
var ErrDedupeUnavailable = errors.New("dedupe backend unavailable")
var compiledPatternCache sync.Map

type Config struct {
	DeliveryPolicy          string
	RetryEnabled            bool
	RetryMaxAttempts        int
	RetryInitialBackoff     time.Duration
	RetryMaxBackoff         time.Duration
	RetryJitter             bool
	FallbackEnabled         bool
	FallbackOnPrimaryFail   bool
	FallbackOnSecondaryFail bool
	FallbackOnPolicyFail    bool
	DLQEnabled              bool
	DLQPath                 string
	DLQOnPrimaryFail        bool
	DLQOnSecondaryFail      bool
	DLQOnFallbackFail       bool
	DLQOnPolicyFail         bool
	DedupeEnabled           bool
	DedupeKey               string
	DedupeWindow            time.Duration
	DedupeBackend           string
	DedupeRedisAddr         string
	DedupeRedisPassword     string
	DedupeRedisDB           int
	DedupeRedisPrefix       string
	ValidateJSONObjects     bool
	OnDiskFull              func()
	OnDLQWrite              func(n int64)
	OnDLQWriteFail          func(n int64)
	OnSchemaWarn            func(err error)
	Schema                  SchemaConfig
	Privacy                 PrivacyConfig
	LogFunc                 func(level string, message string, fields map[string]any)
}

type PrivacyConfig struct {
	Mode       string   // off, warn, enforce
	Blocklist  []string // fields/patterns to redact
	Allowlist  []string // fields/patterns to keep (if set, only these are safe)
	SecretScan bool     // scan for secrets
}

type SchemaConfig struct {
	Mode           string // off, warn, enforce/reject, quarantine
	SchemaVersion  string
	EventVersion   string
	Registry       []SchemaRegistryEntry
	QuarantinePath string
}

type SchemaRegistryEntry struct {
	SchemaVersion  string
	EventVersion   string
	RequiredFields []string
}

type NamedSink struct {
	Name string
	Sink collectorevent.Sink
}

type SinkDeliveryFailure struct {
	Name string
	Err  error
}

type DeliveryOutcome struct {
	PrimaryErr        error
	SecondaryFailures []SinkDeliveryFailure
	SecondarySuccess  int
	FallbackAttempted bool
	FallbackErr       error
	PolicyErr         error
}

func (o DeliveryOutcome) FailureCount() int {
	total := 0
	if o.PrimaryErr != nil {
		total++
	}
	total += len(o.SecondaryFailures)
	if o.FallbackErr != nil {
		total++
	}
	return total
}

func (o DeliveryOutcome) FirstError() error {
	if o.FallbackErr != nil {
		return o.FallbackErr
	}
	if o.PrimaryErr != nil {
		return o.PrimaryErr
	}
	if len(o.SecondaryFailures) > 0 {
		return o.SecondaryFailures[0].Err
	}
	if o.PolicyErr != nil {
		return o.PolicyErr
	}
	return nil
}

type Result struct {
	Accepted   bool
	Invalid    bool
	Quarantined bool
	Deduped    bool
	Outcome    DeliveryOutcome
	Err        error
}

type Processor struct {
	cfg            Config
	primarySink    collectorevent.Sink
	secondarySinks []NamedSink
	fallbackSink   *NamedSink
	rng            *rand.Rand
	dlqFile        *os.File
	dlqMu          sync.Mutex
	quarantineFile *os.File
	quarantineMu   sync.Mutex
	dedupeMu       sync.Mutex
	dedupeSeenAt   map[string]time.Time
	redisClient    *redis.Client
	dedupePrefix   string
}

func New(cfg Config, primary collectorevent.Sink, secondary []NamedSink, fallback *NamedSink, rng *rand.Rand) (*Processor, error) {
	if primary == nil {
		return nil, errors.New("primary sink is required")
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	p := &Processor{
		cfg:            cfg,
		primarySink:    primary,
		secondarySinks: append([]NamedSink(nil), secondary...),
		fallbackSink:   fallback,
		rng:            rng,
		dedupeSeenAt:   make(map[string]time.Time),
		dedupePrefix:   cfg.DedupeRedisPrefix,
	}
	dedupeBackend := strings.ToLower(strings.TrimSpace(cfg.DedupeBackend))
	switch dedupeBackend {
	case "", "memory":
	case "redis":
		if strings.TrimSpace(cfg.DedupeRedisAddr) == "" {
			return nil, errors.New("dedupe redis addr is required when dedupe backend is redis")
		}
		p.redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.DedupeRedisAddr,
			Password: cfg.DedupeRedisPassword,
			DB:       cfg.DedupeRedisDB,
		})
		pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := p.redisClient.Ping(pingCtx).Err(); err != nil {
			_ = p.redisClient.Close()
			return nil, fmt.Errorf("connect redis dedupe store: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported dedupe backend %q", cfg.DedupeBackend)
	}
	if cfg.DLQEnabled {
		if strings.TrimSpace(cfg.DLQPath) == "" {
			return nil, errors.New("dlq path is required when dlq is enabled")
		}
		if err := os.MkdirAll(filepath.Dir(cfg.DLQPath), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir dlq dir: %w", err)
		}
		dlq, err := os.OpenFile(cfg.DLQPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, fmt.Errorf("open dlq file: %w", err)
		}
		p.dlqFile = dlq
	}
	if cfg.Schema.Mode == "quarantine" {
		if strings.TrimSpace(cfg.Schema.QuarantinePath) == "" {
			return nil, errors.New("quarantine path is required when schema mode is quarantine")
		}
		if err := os.MkdirAll(filepath.Dir(cfg.Schema.QuarantinePath), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir quarantine dir: %w", err)
		}
		q, err := os.OpenFile(cfg.Schema.QuarantinePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, fmt.Errorf("open quarantine file: %w", err)
		}
		p.quarantineFile = q
	}
	return p, nil
}

func (p *Processor) Close() error {
	var errs []string
	if p.dlqFile != nil {
		if err := p.dlqFile.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if p.quarantineFile != nil {
		if err := p.quarantineFile.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if p.redisClient != nil {
		if err := p.redisClient.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (p *Processor) Process(ctx context.Context, raw []byte) Result {
	if p.cfg.ValidateJSONObjects && !validation.IsJSONObject(raw) {
		p.writeDLQ(raw, ErrInvalidEvent)
		return Result{Invalid: true, Err: ErrInvalidEvent}
	}

	validated, err := p.validateSchema(raw)
	if err != nil {
		mode := strings.ToLower(p.cfg.Schema.Mode)
		switch mode {
		case "warn":
			if p.cfg.OnSchemaWarn != nil {
				p.cfg.OnSchemaWarn(err)
			} else {
				fmt.Fprintf(os.Stderr, "[WARN] schema validation warning: %v\n", err)
			}
		case "reject", "enforce":
			return Result{Invalid: true, Err: err}
		case "quarantine":
			p.writeQuarantine(raw, err)
			return Result{Accepted: true, Quarantined: true, Err: err}
		}
	}
	if validated != nil {
		raw = validated
	}

	// Apply privacy redaction
	if p.cfg.Privacy.Mode != "" && strings.ToLower(p.cfg.Privacy.Mode) != "off" {
		redacted, redactErr := p.redactPII(raw)
		if redactErr != nil {
			if strings.ToLower(p.cfg.Privacy.Mode) == "enforce" {
				return Result{Invalid: true, Err: fmt.Errorf("privacy redaction failed: %w", redactErr)}
			}
			// warn mode: log and continue
			if p.cfg.LogFunc != nil {
				p.cfg.LogFunc("warn", "privacy_redaction_failed", map[string]any{"error": redactErr.Error()})
			}
		} else {
			raw = redacted
		}
	}

	if p.cfg.DedupeEnabled {
		eventID, ok := validation.ExtractStringPath(raw, p.cfg.DedupeKey)
		if ok {
			duplicate, dedupeErr := p.isDuplicate(eventID)
			if dedupeErr != nil {
				return Result{Err: fmt.Errorf("%w: %v", ErrDedupeUnavailable, dedupeErr)}
			}
			if duplicate {
				return Result{Accepted: true, Deduped: true}
			}
		}
	}

	outcome, err := p.deliverWithRetry(ctx, raw)
	if p.shouldWriteDLQ(outcome) {
		p.writeDLQ(raw, outcome.FirstError())
	}
	if err != nil {
		return Result{Outcome: outcome, Err: err}
	}
	return Result{Accepted: true, Outcome: outcome}
}

func (p *Processor) validateSchema(raw []byte) ([]byte, error) {
	mode := strings.ToLower(p.cfg.Schema.Mode)
	if mode == "" {
		return raw, nil
	}

	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return raw, fmt.Errorf("schema validation: invalid JSON: %w", err)
	}

	modified := false
	sv, _ := data["schema_version"].(string)
	ev, _ := data["event_version"].(string)

	// Normalize: fill in missing version fields from config
	if sv == "" && p.cfg.Schema.SchemaVersion != "" {
		sv = p.cfg.Schema.SchemaVersion
		data["schema_version"] = sv
		modified = true
	}
	if ev == "" && p.cfg.Schema.EventVersion != "" {
		ev = p.cfg.Schema.EventVersion
		data["event_version"] = ev
		modified = true
	}

	if len(p.cfg.Schema.Registry) > 0 {
		found := false
		var entry SchemaRegistryEntry
		for _, e := range p.cfg.Schema.Registry {
			if e.SchemaVersion == sv && e.EventVersion == ev {
				found = true
				entry = e
				break
			}
		}

		if found {
			for _, field := range entry.RequiredFields {
				if _, ok := data[field]; !ok {
					err := fmt.Errorf("schema validation: missing required field %q", field)
					if mode == "off" || mode == "warn" {
						if p.cfg.OnSchemaWarn != nil {
							p.cfg.OnSchemaWarn(err)
						} else {
							fmt.Fprintf(os.Stderr, "[WARN] schema validation warning: %v\n", err)
						}
					}
					if mode == "reject" || mode == "enforce" {
						return raw, err
					}
					if mode == "quarantine" {
						return raw, err
					}
				}
			}
		} else {
			err := fmt.Errorf("schema validation: unknown schema/event version combination (%s/%s)", sv, ev)
			if mode == "reject" || mode == "enforce" || mode == "quarantine" {
				return raw, err
			}
			if mode == "warn" {
				if p.cfg.OnSchemaWarn != nil {
					p.cfg.OnSchemaWarn(err)
				} else {
					fmt.Fprintf(os.Stderr, "[WARN] schema validation warning: %v\n", err)
				}
			}
		}
	}

	if modified {
		newRaw, err := json.Marshal(data)
		if err != nil {
			return raw, err
		}
		return newRaw, nil
	}
	return raw, nil
}

func (p *Processor) writeQuarantine(raw []byte, err error) {
	if p.quarantineFile == nil {
		return
	}
	errMsg := "schema_validation_failed"
	if err != nil {
		errMsg = err.Error()
	}
	record := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"error":     errMsg,
		"raw":       string(raw),
	}
	b, mErr := json.Marshal(record)
	if mErr != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] quarantine marshal failed: %v, raw event lost: %s\n", mErr, string(raw))
		return
	}

	// Copy data under the lock, then release before sleeping on retry.
	data := make([]byte, len(b)+1)
	copy(data, b)
	data[len(b)] = '\n'

	p.quarantineMu.Lock()
	_, writeErr := p.quarantineFile.Write(data)
	p.quarantineMu.Unlock()

	if writeErr == nil {
		return
	}

	// Retry without holding the lock so concurrent writers are not blocked.
	const maxRetries = 3
	var lastWriteErr error
	for attempt := 1; attempt < maxRetries; attempt++ {
		time.Sleep(10 * time.Millisecond * time.Duration(attempt+1))
		p.quarantineMu.Lock()
		_, lastWriteErr = p.quarantineFile.Write(data)
		p.quarantineMu.Unlock()
		if lastWriteErr == nil {
			return
		}
	}

	fmt.Fprintf(os.Stderr, "[FATAL] quarantine write failed after %d attempts: %v, event: %s\n", maxRetries, lastWriteErr, string(raw))
}

func (p *Processor) normalizedDeliveryPolicy() string {
	policy := strings.ToLower(strings.TrimSpace(p.cfg.DeliveryPolicy))
	if policy == "" {
		return "require_primary"
	}
	return policy
}

func (p *Processor) evaluatePolicy(outcome *DeliveryOutcome) {
	primaryOK := outcome.PrimaryErr == nil
	switch p.normalizedDeliveryPolicy() {
	case "best_effort":
		if primaryOK || outcome.SecondarySuccess > 0 {
			outcome.PolicyErr = nil
			return
		}
	case "require_all":
		if primaryOK && len(outcome.SecondaryFailures) == 0 && outcome.SecondarySuccess == len(p.secondarySinks) {
			outcome.PolicyErr = nil
			return
		}
	default:
		if primaryOK {
			outcome.PolicyErr = nil
			return
		}
	}

	outcome.PolicyErr = fmt.Errorf(
		"delivery policy %s failed (primary_failed=%t secondary_failures=%d secondary_success=%d)",
		p.normalizedDeliveryPolicy(),
		outcome.PrimaryErr != nil,
		len(outcome.SecondaryFailures),
		outcome.SecondarySuccess,
	)
}

func (p *Processor) shouldAttemptFallback(outcome DeliveryOutcome) bool {
	if !p.cfg.FallbackEnabled || p.fallbackSink == nil {
		return false
	}
	if outcome.PrimaryErr != nil && p.cfg.FallbackOnPrimaryFail {
		return true
	}
	if len(outcome.SecondaryFailures) > 0 && p.cfg.FallbackOnSecondaryFail {
		return true
	}
	if outcome.PolicyErr != nil && p.cfg.FallbackOnPolicyFail {
		return true
	}
	return false
}

func (p *Processor) shouldWriteDLQ(outcome DeliveryOutcome) bool {
	if !p.cfg.DLQEnabled {
		return false
	}
	if outcome.PrimaryErr != nil && p.cfg.DLQOnPrimaryFail {
		return true
	}
	if len(outcome.SecondaryFailures) > 0 && p.cfg.DLQOnSecondaryFail {
		return true
	}
	if outcome.FallbackErr != nil && p.cfg.DLQOnFallbackFail {
		return true
	}
	if outcome.PolicyErr != nil && p.cfg.DLQOnPolicyFail {
		return true
	}
	return false
}

func (p *Processor) runDeliveryAttempt(ctx context.Context, raw []byte) DeliveryOutcome {
	outcome := DeliveryOutcome{}
	if err := p.primarySink.WriteEvent(ctx, raw, nil); err != nil {
		outcome.PrimaryErr = err
		if isDiskFullErr(err) && p.cfg.OnDiskFull != nil {
			p.cfg.OnDiskFull()
		}
	}

	for _, secondary := range p.secondarySinks {
		if err := secondary.Sink.WriteEvent(ctx, raw, nil); err != nil {
			outcome.SecondaryFailures = append(outcome.SecondaryFailures, SinkDeliveryFailure{Name: secondary.Name, Err: err})
			if isDiskFullErr(err) && p.cfg.OnDiskFull != nil {
				p.cfg.OnDiskFull()
			}
			continue
		}
		outcome.SecondarySuccess++
	}

	p.evaluatePolicy(&outcome)

	if p.shouldAttemptFallback(outcome) {
		outcome.FallbackAttempted = true
		if err := p.fallbackSink.Sink.WriteEvent(ctx, raw, nil); err != nil {
			outcome.FallbackErr = err
			if isDiskFullErr(err) && p.cfg.OnDiskFull != nil {
				p.cfg.OnDiskFull()
			}
		}
	}

	return outcome
}

func (p *Processor) deliverWithRetry(ctx context.Context, raw []byte) (DeliveryOutcome, error) {
	attempts := 1
	if p.cfg.RetryEnabled {
		attempts = p.cfg.RetryMaxAttempts
		if attempts <= 0 {
			attempts = 1
		}
	}
	backoff := p.cfg.RetryInitialBackoff
	maxBackoff := p.cfg.RetryMaxBackoff

	var last DeliveryOutcome
	for i := 0; i < attempts; i++ {
		last = p.runDeliveryAttempt(ctx, raw)
		if last.PolicyErr == nil {
			return last, nil
		}
		if i == attempts-1 {
			break
		}
		sleep := backoff
		if sleep > 0 && p.cfg.RetryJitter {
			sleep = time.Duration(float64(sleep) * (0.8 + 0.4*p.rng.Float64()))
		}
		if sleep > 0 {
			if err := sleepContext(ctx, sleep); err != nil {
				return last, err
			}
		}
		if backoff > 0 {
			backoff *= 2
		}
		if maxBackoff > 0 && backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
	if last.PolicyErr == nil {
		last.PolicyErr = errors.New("delivery failed")
	}
	return last, last.PolicyErr
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (p *Processor) writeDLQ(raw []byte, err error) {
	if !p.cfg.DLQEnabled || p.dlqFile == nil {
		return
	}
	errMsg := "delivery_failed"
	if err != nil {
		errMsg = err.Error()
	}
	record := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"error":     errMsg,
		"raw":       string(raw),
	}
	b, mErr := json.Marshal(record)
	if mErr != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] DLQ marshal failed: %v, raw event lost: %s\n", mErr, string(raw))
		return
	}

	// Copy data under the lock, then release before sleeping on retry.
	data := make([]byte, len(b)+1)
	copy(data, b)
	data[len(b)] = '\n'

	p.dlqMu.Lock()
	_, writeErr := p.dlqFile.Write(data)
	p.dlqMu.Unlock()

	if writeErr == nil {
		if p.cfg.OnDLQWrite != nil {
			p.cfg.OnDLQWrite(1)
		}
		return
	}

	// Retry without holding the lock so concurrent writers are not blocked.
	const maxRetries = 3
	var lastWriteErr error
	for attempt := 1; attempt < maxRetries; attempt++ {
		time.Sleep(10 * time.Millisecond * time.Duration(attempt+1))
		p.dlqMu.Lock()
		_, lastWriteErr = p.dlqFile.Write(data)
		p.dlqMu.Unlock()
		if lastWriteErr == nil {
			if p.cfg.OnDLQWrite != nil {
				p.cfg.OnDLQWrite(1)
			}
			return
		}
	}

	// All retries failed - this is a critical failure
	fmt.Fprintf(os.Stderr, "[FATAL] DLQ write failed after %d attempts: %v, event: %s\n", maxRetries, lastWriteErr, string(raw))
	if p.cfg.OnDLQWriteFail != nil {
		p.cfg.OnDLQWriteFail(1)
	}
}

// maxDedupeEntries caps the in-memory dedupe map to prevent unbounded growth.
const maxDedupeEntries = 100_000

func (p *Processor) isDuplicate(value string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return false, nil
	}
	if p.redisClient != nil {
		key := value
		if strings.TrimSpace(p.dedupePrefix) != "" {
			key = p.dedupePrefix + value
		}
		ok, err := p.redisClient.SetNX(context.Background(), key, "1", p.cfg.DedupeWindow).Result()
		if err != nil {
			return false, err
		}
		return !ok, nil
	}
	p.dedupeMu.Lock()
	defer p.dedupeMu.Unlock()
	now := time.Now()
	for k, ts := range p.dedupeSeenAt {
		if now.Sub(ts) > p.cfg.DedupeWindow {
			delete(p.dedupeSeenAt, k)
		}
	}
	if ts, ok := p.dedupeSeenAt[value]; ok && now.Sub(ts) <= p.cfg.DedupeWindow {
		return true, nil
	}
	// Evict oldest half if the map exceeds the cap.
	if len(p.dedupeSeenAt) >= maxDedupeEntries {
		count := 0
		for k := range p.dedupeSeenAt {
			delete(p.dedupeSeenAt, k)
			count++
			if count >= maxDedupeEntries/2 {
				break
			}
		}
	}
	p.dedupeSeenAt[value] = now
	return false, nil
}

// WriteDLQ writes a raw event to the DLQ if DLQ is enabled.
// This is exposed so that spool delivery can fall back to DLQ on failure.
func (p *Processor) WriteDLQ(raw []byte, err error) {
	if !p.cfg.DLQEnabled || p.dlqFile == nil {
		return
	}
	dlqErr := err
	if dlqErr == nil {
		dlqErr = fmt.Errorf("spool delivery error")
	}
	p.writeDLQ(raw, dlqErr)
}

// isDiskFullErr checks whether an error indicates a full disk.
// Also used by the collector main package via a thin wrapper.
func isDiskFullErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "no space left") || strings.Contains(s, "disk full")
}

// IsDiskFullErr is the exported version for use outside this package.
func IsDiskFullErr(err error) bool { return isDiskFullErr(err) }

// redactPII applies privacy redaction to a JSON event
func (p *Processor) redactPII(raw []byte) ([]byte, error) {
	if p.cfg.Privacy.Mode == "" || strings.ToLower(p.cfg.Privacy.Mode) == "off" {
		return raw, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return raw, err
	}

	redacted := p.redactMap(data, "")
	result, err := json.Marshal(redacted)
	if err != nil {
		return raw, err
	}

	return result, nil
}

// redactMap recursively redacts sensitive fields in a map
func (p *Processor) redactMap(data map[string]interface{}, prefix string) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range data {
		fieldPath := key
		if prefix != "" {
			fieldPath = prefix + "." + key
		}

		if p.shouldRedactField(fieldPath, key) {
			result[key] = "[REDACTED]"
			if p.cfg.LogFunc != nil {
				p.cfg.LogFunc("info", "pii_redacted", map[string]any{"field": fieldPath})
			}
		} else if nested, ok := value.(map[string]interface{}); ok {
			result[key] = p.redactMap(nested, fieldPath)
		} else if arr, ok := value.([]interface{}); ok {
			result[key] = p.redactArray(arr, fieldPath)
		} else {
			result[key] = value
		}
	}

	return result
}

// redactArray recursively redacts sensitive fields in an array
func (p *Processor) redactArray(arr []interface{}, prefix string) []interface{} {
	result := make([]interface{}, len(arr))

	for i, item := range arr {
		if nested, ok := item.(map[string]interface{}); ok {
			result[i] = p.redactMap(nested, prefix)
		} else if nestedArr, ok := item.([]interface{}); ok {
			result[i] = p.redactArray(nestedArr, prefix)
		} else {
			result[i] = item
		}
	}

	return result
}

// shouldRedactField determines if a field should be redacted
func (p *Processor) shouldRedactField(fieldPath, key string) bool {
	keyLower := strings.ToLower(key)

	// If allowlist is specified, redact everything NOT in allowlist
	if len(p.cfg.Privacy.Allowlist) > 0 {
		allowed := false
		for _, pattern := range p.cfg.Privacy.Allowlist {
			if p.matchesPattern(fieldPath, keyLower, pattern) {
				allowed = true
				break
			}
		}
		return !allowed // Redact if NOT in allowlist
	}

	// Check blocklist
	for _, pattern := range p.cfg.Privacy.Blocklist {
		if p.matchesPattern(fieldPath, keyLower, pattern) {
			return true
		}
	}

	return false
}

// matchesPattern checks if a field matches a redaction pattern
func (p *Processor) matchesPattern(fieldPath, keyLower, pattern string) bool {
	patternLower := strings.ToLower(pattern)

	// Exact match on key
	if keyLower == patternLower {
		return true
	}

	// Match on full path
	if fieldPath == pattern || strings.ToLower(fieldPath) == patternLower {
		return true
	}

	// Match last segment of path (e.g., "email" matches "user.email")
	parts := strings.Split(fieldPath, ".")
	if len(parts) > 0 && strings.ToLower(parts[len(parts)-1]) == patternLower {
		return true
	}

	// Wildcard match (e.g., "user.*" matches "user.email")
	if strings.Contains(patternLower, "*") {
		wildcardRegex := strings.ReplaceAll(patternLower, "*", ".*")
		patternKey := "^" + wildcardRegex + "$"
		var re *regexp.Regexp
		if cached, ok := compiledPatternCache.Load(patternKey); ok {
			re = cached.(*regexp.Regexp)
		} else {
			re = regexp.MustCompile(patternKey)
			compiledPatternCache.Store(patternKey, re)
		}
		if match := re.MatchString(fieldPath); match {
			return true
		}
	}

	return false
}
