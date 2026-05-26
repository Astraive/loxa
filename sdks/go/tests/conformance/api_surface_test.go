package conformance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/astraive/loxa/sdks/go"
	"github.com/astraive/loxa/sdks/go/src/testkit"
)

func TestPublicAPISurfaceCoreFlow(t *testing.T) {
	sink, store := loxa.MemorySink()
	cfg := loxa.Test().
		WithService("checkout").
		WithVersion("1.2.0").
		WithEnvironment("prod").
		WithSink(sink).
		WithSampler(loxa.SampleAll()).
		WithRedactor(loxa.ComposeRedactors(loxa.DefaultRedactor(), loxa.MaskKeys("user.email"))).
		WithAsync(false)
	cfg.Security = loxa.SecurityConfig{
		RedactByDefault:     true,
		AllowPII:            false,
		MaxFieldBytes:       4096,
		MaxEventBytes:       262144,
		MaxAttrCount:        512,
		DropOversizedEvents: true,
	}
	if err := loxa.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Service:     "checkout",
		Version:     "1.2.0",
		Environment: "prod",
		Region:      "ap-south-1",
		Name:        "checkout_request",
		Kind:        "http",
		Level:       loxa.LevelInfo,
		RequestID:   "req_123",
		TraceID:     "trace_abc",
		SpanID:      "span_abc",
		Method:      "POST",
		Path:        "/checkout",
		Route:       "/checkout",
		Host:        "api.example.com",
		UserID:      "u_123",
		TenantID:    "t_123",
		WorkspaceID: "w_123",
		Custom: []loxa.Attr{
			loxa.String("feature.checkout_v2", "enabled"),
		},
	})
	if !loxa.HasEvent(ctx) {
		t.Fatalf("expected event in context")
	}
	if loxa.EventID(ctx) == "" {
		t.Fatalf("expected event id")
	}
	if loxa.RequestIDFromContext(ctx) == "" {
		t.Fatalf("expected request id")
	}
	if loxa.TraceIDFromContext(ctx) == "" {
		t.Fatalf("expected trace id")
	}
	if loxa.SpanIDFromContext(ctx) == "" {
		t.Fatalf("expected span id")
	}

	_ = loxa.Enrich(ctx,
		loxa.String("user.id", "u_123"),
		loxa.Int("cart.items", 3),
		loxa.Bool("payment.retry", false),
		loxa.Null("optional.note"),
		loxa.Group("user", loxa.String("id", "u_123"), loxa.String("email", "a@example.com")),
		loxa.UserID("u_123"),
		loxa.TenantID("t_456"),
		loxa.WorkspaceID("w_123"),
		loxa.OrganizationID("o_123"),
		loxa.SessionID("s_123"),
		loxa.FeatureFlag("checkout_v2", "on"),
		loxa.FeatureFlagBool("new_ui", true),
		loxa.Experiment("pricing_test", "variant_b"),
		loxa.OrderID("ord_123"),
		loxa.CartID("cart_1"),
		loxa.ProductID("prod_123"),
		loxa.CustomerID("cus_123"),
		loxa.Plan("pro"),
		loxa.Currency("INR"),
		loxa.Amount(4999),
		loxa.Country("IN"),
		loxa.Device("desktop"),
		loxa.Platform("web"),
		loxa.AppVersion("0.2.0"),
		loxa.ErrorType("ValidationError"),
		loxa.ErrorCode("INVALID_INPUT"),
		loxa.ErrorMessage("validation failed"),
		loxa.ErrorStack("stack"),
		loxa.Retryable(true),
		loxa.JobName("send_email"),
		loxa.QueueName("emails"),
		loxa.MessageID("msg_123"),
		loxa.Attempt(2),
		loxa.SensitiveString("user.email", "person@example.com"),
		loxa.MarkSensitive(loxa.String("auth.token", "secret")),
		loxa.HashString("user.email", "person@example.com"),
	)
	_ = loxa.Checkpoint(ctx, "payment_started")
	_ = loxa.Finish(ctx, "success", loxa.Int("status_code", 200))
	if err := loxa.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := loxa.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if store.Len() == 0 {
		t.Fatalf("expected emitted event")
	}
}

func TestLifecycleShortcutsAndManualEventAPI(t *testing.T) {
	sink, _ := loxa.MemorySink()
	if err := loxa.Configure(loxa.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	_ = loxa.StartHTTPEvent(context.Background(), loxa.Params{Method: "GET", Path: "/x"})
	_ = loxa.StartJobEvent(context.Background(), loxa.Params{Event: "job.run"})
	_ = loxa.StartQueueEvent(context.Background(), loxa.Params{Event: "queue.process"})
	_ = loxa.StartCLIEvent(context.Background(), loxa.Params{Event: "cli.run"})
	_ = loxa.StartCronEvent(context.Background(), loxa.Params{Event: "cron.run"})
	_ = loxa.StartJob(context.Background(), "send_email")
	_ = loxa.StartQueueJob(context.Background(), "emails", "msg_123")
	_ = loxa.StartCron(context.Background(), "daily_billing")

	ev := loxa.NewEvent(loxa.Params{Event: "manual.event"})
	_ = ev.Enrich(loxa.String("k", "v"))
	_ = ev.Checkpoint("cp")
	_ = ev.Finish("success")
	if err := loxa.EmitEvent(ev); err != nil {
		t.Fatalf("emit event: %v", err)
	}
	if !ev.IsFinished() {
		t.Fatalf("expected finished event")
	}
}

func TestImmediateLoggerAndHelpers(t *testing.T) {
	sink, store := loxa.MemorySink()
	if err := loxa.Configure(loxa.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	loxa.Debug("debug msg")
	loxa.Info("info msg", loxa.String("queue", "emails"))
	loxa.Warn("warn msg")
	loxa.Error("error msg")
	loxa.DebugContext(context.Background(), "debug ctx", "debug.event")
	loxa.InfoContext(context.Background(), "info ctx", "info.event")
	loxa.WarnContext(context.Background(), "warn ctx", "warn.event")
	loxa.ErrorContext(context.Background(), "error ctx", nil, "error.event")

	lg, err := loxa.New(loxa.Test().WithSink(sink))
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := lg.StartEvent(context.Background(), loxa.Params{Event: "logger.event"})
	_ = lg.Enrich(ctx, loxa.String("user.id", "u_1"))
	_ = lg.Checkpoint(ctx, "db_started")
	_ = lg.Finish(ctx, "success")
	if err := lg.Emit(ctx); err != nil {
		t.Fatalf("logger emit: %v", err)
	}
	lg.Debug("dbg")
	lg.Info("inf")
	lg.Warn("wrn")
	lg.Error("err")

	captured, err := testkit.Capture(func() {
		cctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "capture.event"})
		_ = loxa.Enrich(cctx, loxa.String("a", "b"))
		_ = loxa.Finish(cctx, "success")
		_ = loxa.Emit(cctx)
	})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if len(captured) == 0 || store.Len() == 0 {
		t.Fatalf("expected captured and emitted logs")
	}
	testkit.AssertEvent(t, captured[0], "a", "b")
}

func TestHTTPClientInstrumentation(t *testing.T) {
	sink, store := loxa.MemorySink()
	if err := loxa.Configure(loxa.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer downstream.Close()

	client := loxa.WrapHTTPClient(&http.Client{Timeout: 2 * time.Second})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api", func(w http.ResponseWriter, r *http.Request) {
		ctx := loxa.StartHTTPEvent(r.Context(), loxa.Params{
			Event:  "http.request",
			Method: r.Method,
			Path:   r.URL.Path,
			Route:  "/api",
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, downstream.URL, nil)
		_, _ = client.Do(req)
		_ = loxa.Enrich(ctx, loxa.UserID("u_1"))
		_ = loxa.Finish(ctx, "success", loxa.Int("status_code", 200))
		_ = loxa.Emit(ctx)
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api")
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	_ = resp.Body.Close()

	if store.Len() == 0 {
		t.Fatalf("expected emitted events")
	}
}
