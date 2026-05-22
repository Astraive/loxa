package nethttp

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/astraive/loxa/sdks/go"
)

func TestMiddlewarePanicBeforeWriteReturns500AndErrorEvent(t *testing.T) {
	store := configureMemoryStore(t)
	h := Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom-before-write")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, http.StatusText(http.StatusInternalServerError)) {
		t.Fatalf("expected 500 response body, got %q", body)
	}
	ev := singleEvent(t, store)
	if ev.Outcome != "error" {
		t.Fatalf("expected error outcome, got %q", ev.Outcome)
	}
	if ev.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected event status 500, got %d", ev.StatusCode)
	}
	if got := mustInt64Attr(t, ev, "response_bytes"); got <= 0 {
		t.Fatalf("expected response bytes > 0, got %d", got)
	}
	if ev.Error == nil || !strings.Contains(ev.Error.Message, "boom-before-write") {
		t.Fatalf("expected panic error with panic value, got %+v", ev.Error)
	}
}

func TestMiddlewarePanicAfterWritePreservesResponseAndEventStatus(t *testing.T) {
	store := configureMemoryStore(t)
	h := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "partial")
		panic("boom-after-write")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected response status to remain 202, got %d", rr.Code)
	}
	if got := rr.Body.String(); got != "partial" {
		t.Fatalf("expected partial body preserved, got %q", got)
	}
	ev := singleEvent(t, store)
	if ev.Outcome != "error" {
		t.Fatalf("expected error outcome, got %q", ev.Outcome)
	}
	if ev.StatusCode != http.StatusAccepted {
		t.Fatalf("expected event status 202, got %d", ev.StatusCode)
	}
	if got := mustInt64Attr(t, ev, "response_bytes"); got != int64(len("partial")) {
		t.Fatalf("expected response bytes %d, got %d", len("partial"), got)
	}
}

func TestMiddlewareCapturesRouteAndSelectedHeaders(t *testing.T) {
	store := configureMemoryStore(t)
	extractorCalls := 0
	h := MiddlewareWithConfig(Config{
		RouteExtractor: func(*http.Request) string {
			extractorCalls++
			return "/users/{id}"
		},
		HeaderAttrs: []string{"X-Tenant-ID", "X_CUSTOM_HEADER", "X-Empty", " ", ""},
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req.Header.Set("X-Tenant-ID", "t-1")
	req.Header.Set("X_CUSTOM_HEADER", "value-1")
	req.Header.Set("X-Empty", "   ")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if extractorCalls != 1 {
		t.Fatalf("expected route extractor called once, got %d", extractorCalls)
	}
	ev := singleEvent(t, store)
	if ev.Route != "/users/{id}" {
		t.Fatalf("expected route template, got %q", ev.Route)
	}
	tenant, ok := ev.Get("http.header.x-tenant-id")
	if !ok || tenant != "t-1" {
		t.Fatalf("expected selected tenant header, got %v", tenant)
	}
	custom, ok := ev.Get("http.header.x-custom-header")
	if !ok || custom != "value-1" {
		t.Fatalf("expected normalized custom header, got %v", custom)
	}
	if _, ok := ev.Get("http.header.x-empty"); ok {
		t.Fatalf("expected empty header value to be omitted")
	}
}

func TestMiddlewareForwardedIPTrustChainHandling(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		remote  string
		headers map[string]string
		wantIP  string
	}{
		{
			name:   "trust disabled ignores forwarded chain",
			cfg:    Config{TrustForwardedFor: false},
			remote: "10.2.3.4:9999",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.9, 203.0.113.10",
			},
			wantIP: "10.2.3.4",
		},
		{
			name:   "trust enabled picks first valid ip from chain",
			cfg:    Config{TrustForwardedFor: true},
			remote: "10.1.1.2:1234",
			headers: map[string]string{
				"X-Forwarded-For": "unknown, 203.0.113.5, 10.0.0.1",
			},
			wantIP: "203.0.113.5",
		},
		{
			name:   "trust enabled parses host port candidate",
			cfg:    Config{TrustForwardedFor: true},
			remote: "10.1.1.2:1234",
			headers: map[string]string{
				"X-Forwarded-For": "198.51.100.77:8443, 10.0.0.1",
			},
			wantIP: "198.51.100.77",
		},
		{
			name:   "trust enabled supports custom forwarded header",
			cfg:    Config{TrustForwardedFor: true, ForwardedForHeader: "Forwarded"},
			remote: "10.1.1.2:1234",
			headers: map[string]string{
				"Forwarded": `for="203.0.113.60";proto=https, for=10.0.0.1`,
			},
			wantIP: "203.0.113.60",
		},
		{
			name:   "invalid forwarded chain falls back to remote",
			cfg:    Config{TrustForwardedFor: true},
			remote: "10.9.8.7:4321",
			headers: map[string]string{
				"X-Forwarded-For": "unknown, invalid-ip",
			},
			wantIP: "10.9.8.7",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := configureMemoryStore(t)
			h := MiddlewareWithConfig(tc.cfg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, "ok")
			}))

			req := httptest.NewRequest(http.MethodGet, "/ip", nil)
			req.RemoteAddr = tc.remote
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			ev := singleEvent(t, store)
			ip, _ := ev.Get("remote_ip")
			if ip != tc.wantIP {
				t.Fatalf("expected remote_ip %q, got %v", tc.wantIP, ip)
			}
		})
	}
}

func TestResponseWriterPreservesOptionalInterfaceContracts(t *testing.T) {
	tests := []struct {
		name       string
		writer     http.ResponseWriter
		flusher    bool
		hijacker   bool
		pusher     bool
		readerFrom bool
	}{
		{name: "base", writer: newBaseTestWriter()},
		{name: "flusher", writer: &flusherWriter{baseTestWriter: newBaseTestWriter()}, flusher: true},
		{name: "hijacker", writer: &hijackerWriter{baseTestWriter: newBaseTestWriter()}, hijacker: true},
		{name: "pusher", writer: &pusherWriter{baseTestWriter: newBaseTestWriter()}, pusher: true},
		{name: "reader-from", writer: &readerFromWriter{baseTestWriter: newBaseTestWriter()}, readerFrom: true},
		{name: "flusher-pusher", writer: &flusherPusherWriter{baseTestWriter: newBaseTestWriter()}, flusher: true, pusher: true},
		{name: "full", writer: &fullFeaturedWriter{baseTestWriter: newBaseTestWriter()}, flusher: true, hijacker: true, pusher: true, readerFrom: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wrapped, state := newResponseWriter(tc.writer)

			_, gotFlusher := wrapped.(http.Flusher)
			_, gotHijacker := wrapped.(http.Hijacker)
			_, gotPusher := wrapped.(http.Pusher)
			_, gotReaderFrom := wrapped.(io.ReaderFrom)
			if gotFlusher != tc.flusher || gotHijacker != tc.hijacker || gotPusher != tc.pusher || gotReaderFrom != tc.readerFrom {
				t.Fatalf("unexpected optional interface set: got(flusher=%t,hijacker=%t,pusher=%t,readerFrom=%t) want(flusher=%t,hijacker=%t,pusher=%t,readerFrom=%t)",
					gotFlusher, gotHijacker, gotPusher, gotReaderFrom, tc.flusher, tc.hijacker, tc.pusher, tc.readerFrom)
			}

			if tc.readerFrom {
				if _, err := wrapped.(io.ReaderFrom).ReadFrom(strings.NewReader("abc")); err != nil {
					t.Fatalf("ReadFrom failed: %v", err)
				}
			} else {
				if _, err := wrapped.Write([]byte("abc")); err != nil {
					t.Fatalf("Write failed: %v", err)
				}
			}
			if state.bytes != 3 {
				t.Fatalf("expected bytes tracked as 3, got %d", state.bytes)
			}
		})
	}
}

func configureMemoryStore(t *testing.T) *loxa.MemorySinkStore {
	t.Helper()
	sink, store := loxa.MemorySink()
	if err := loxa.Configure(loxa.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	return store
}

func singleEvent(t *testing.T, store *loxa.MemorySinkStore) *loxa.Event {
	t.Helper()
	if store.Len() != 1 {
		t.Fatalf("expected 1 event, got %d", store.Len())
	}
	return store.Events()[0]
}

func mustInt64Attr(t *testing.T, ev *loxa.Event, key string) int64 {
	t.Helper()
	v, ok := ev.Get(key)
	if !ok {
		t.Fatalf("expected attr %q to exist", key)
	}
	switch n := v.(type) {
	case int:
		return int64(n)
	case int8:
		return int64(n)
	case int16:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	default:
		t.Fatalf("expected numeric attr for %q, got %T", key, v)
		return 0
	}
}

type baseTestWriter struct {
	header http.Header
}

func newBaseTestWriter() *baseTestWriter {
	return &baseTestWriter{header: make(http.Header)}
}

func (w *baseTestWriter) Header() http.Header { return w.header }
func (w *baseTestWriter) WriteHeader(int)     {}
func (w *baseTestWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

type flusherWriter struct{ *baseTestWriter }

func (w *flusherWriter) Flush() {}

type hijackerWriter struct{ *baseTestWriter }

func (w *hijackerWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, context.Canceled
}

type pusherWriter struct{ *baseTestWriter }

func (w *pusherWriter) Push(string, *http.PushOptions) error { return nil }

type readerFromWriter struct{ *baseTestWriter }

func (w *readerFromWriter) ReadFrom(r io.Reader) (int64, error) { return io.Copy(io.Discard, r) }

type flusherPusherWriter struct{ *baseTestWriter }

func (w *flusherPusherWriter) Flush()                               {}
func (w *flusherPusherWriter) Push(string, *http.PushOptions) error { return nil }

type fullFeaturedWriter struct{ *baseTestWriter }

func (w *fullFeaturedWriter) Flush() {}
func (w *fullFeaturedWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, context.Canceled
}
func (w *fullFeaturedWriter) Push(string, *http.PushOptions) error { return nil }
func (w *fullFeaturedWriter) ReadFrom(r io.Reader) (int64, error)  { return io.Copy(io.Discard, r) }
