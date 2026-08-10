package conformance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/astraive/loza/sdks/go"
	"github.com/astraive/loza/sdks/go/src/testkit"
)

func TestPublicAPISurfaceCoreFlow(t *testing.T) {
	sink, store := loza.MemorySink()
	cfg := loza.Test().
		WithService("checkout").
		WithVersion("1.2.0").
		WithEnvironment("prod").
		WithSink(sink).
		WithSampler(loza.SampleAll()).
		WithRedactor(loza.ComposeRedactors(loza.DefaultRedactor(), loza.MaskKeys("user.email"))).
		WithAsync(false)
	cfg.Security = loza.SecurityConfig{
		RedactByDefault:     true,
		AllowPII:            false,
		MaxFieldBytes:       4096,
		MaxEventBytes:       262144,
		MaxAttrCount:        512,
		DropOversizedEvents: true,
	}
	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	ctx := loza.StartEvent(context.Background(), loza.Params{
		Service:     "checkout",
		Version:     "1.2.0",
		Environment: "prod",
		Region:      "ap-south-1",
		Name:        "checkout_request",
		Kind:        "http",
		Level:       loza.LevelInfo,
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
		Custom: []loza.Attr{
			loza.String("feature.checkout_v2", "enabled"),
		},
	})
	if !loza.HasEvent(ctx) {
		t.Fatalf("expected event in context")
	}
	if loza.EventID(ctx) == "" {
		t.Fatalf("expected event id")
	}
	if loza.RequestIDFromContext(ctx) == "" {
		t.Fatalf("expected request id")
	}
	if loza.TraceIDFromContext(ctx) == "" {
		t.Fatalf("expected trace id")
	}
	if loza.SpanIDFromContext(ctx) == "" {
		t.Fatalf("expected span id")
	}

	_ = loza.Enrich(ctx,
		loza.String("user.id", "u_123"),
		loza.Int("cart.items", 3),
		loza.Bool("payment.retry", false),
		loza.Null("optional.note"),
		loza.Group("user", loza.String("id", "u_123"), loza.String("email", "a@example.com")),
		loza.UserID("u_123"),
		loza.TenantID("t_456"),
		loza.WorkspaceID("w_123"),
		loza.OrganizationID("o_123"),
		loza.SessionID("s_123"),
		loza.FeatureFlag("checkout_v2", "on"),
		loza.FeatureFlagBool("new_ui", true),
		loza.Experiment("pricing_test", "variant_b"),
		loza.OrderID("ord_123"),
		loza.CartID("cart_1"),
		loza.ProductID("prod_123"),
		loza.CustomerID("cus_123"),
		loza.Plan("pro"),
		loza.Currency("INR"),
		loza.Amount(4999),
		loza.Country("IN"),
		loza.Device("desktop"),
		loza.Platform("web"),
		loza.AppVersion("0.2.0"),
		loza.ErrorType("ValidationError"),
		loza.ErrorCode("INVALID_INPUT"),
		loza.ErrorMessage("validation failed"),
		loza.ErrorStack("stack"),
		loza.Retryable(true),
		loza.JobName("send_email"),
		loza.QueueName("emails"),
		loza.MessageID("msg_123"),
		loza.Attempt(2),
		loza.SensitiveString("user.email", "person@example.com"),
		loza.MarkSensitive(loza.String("auth.token", "secret")),
		loza.HashString("user.email", "person@example.com"),
	)
	_ = loza.Checkpoint(ctx, "payment_started")
	_ = loza.Finish(ctx, "success", loza.Int("status_code", 200))
	if err := loza.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if store.Len() == 0 {
		t.Fatalf("expected emitted event")
	}
}

func TestLifecycleShortcutsAndManualEventAPI(t *testing.T) {
	sink, _ := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	_ = loza.StartHTTPEvent(context.Background(), loza.Params{Method: "GET", Path: "/x"})
	_ = loza.StartJobEvent(context.Background(), loza.Params{Event: "job.run"})
	_ = loza.StartQueueEvent(context.Background(), loza.Params{Event: "queue.process"})
	_ = loza.StartCLIEvent(context.Background(), loza.Params{Event: "cli.run"})
	_ = loza.StartCronEvent(context.Background(), loza.Params{Event: "cron.run"})
	_ = loza.StartJob(context.Background(), "send_email")
	_ = loza.StartQueueJob(context.Background(), "emails", "msg_123")
	_ = loza.StartCron(context.Background(), "daily_billing")

	ev := loza.NewEvent(loza.Params{Event: "manual.event"})
	_ = ev.Enrich(loza.String("k", "v"))
	_ = ev.Checkpoint("cp")
	_ = ev.Finish("success")
	if err := loza.EmitEvent(ev); err != nil {
		t.Fatalf("emit event: %v", err)
	}
	if !ev.IsFinished() {
		t.Fatalf("expected finished event")
	}
}

func TestImmediateLoggerAndHelpers(t *testing.T) {
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	loza.Debug("debug msg")
	loza.Info("info msg", loza.String("queue", "emails"))
	loza.Warn("warn msg")
	loza.Error("error msg")
	loza.DebugContext(context.Background(), "debug ctx", "debug.event")
	loza.InfoContext(context.Background(), "info ctx", "info.event")
	loza.WarnContext(context.Background(), "warn ctx", "warn.event")
	loza.ErrorContext(context.Background(), "error ctx", nil, "error.event")

	lg, err := loza.New(loza.Test().WithSink(sink))
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	ctx := lg.StartEvent(context.Background(), loza.Params{Event: "logger.event"})
	_ = lg.Enrich(ctx, loza.String("user.id", "u_1"))
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
		cctx := loza.StartEvent(context.Background(), loza.Params{Event: "capture.event"})
		_ = loza.Enrich(cctx, loza.String("a", "b"))
		_ = loza.Finish(cctx, "success")
		_ = loza.Emit(cctx)
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
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer downstream.Close()

	client := loza.WrapHTTPClient(&http.Client{Timeout: 2 * time.Second})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api", func(w http.ResponseWriter, r *http.Request) {
		ctx := loza.StartHTTPEvent(r.Context(), loza.Params{
			Event:  "http.request",
			Method: r.Method,
			Path:   r.URL.Path,
			Route:  "/api",
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, downstream.URL, nil)
		_, _ = client.Do(req)
		_ = loza.Enrich(ctx, loza.UserID("u_1"))
		_ = loza.Finish(ctx, "success", loza.Int("status_code", 200))
		_ = loza.Emit(ctx)
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
