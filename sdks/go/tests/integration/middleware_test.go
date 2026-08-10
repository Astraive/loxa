package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astraive/loza/sdks/go"
	lozahttp "github.com/astraive/loza/sdks/go/src/middleware/nethttp"
)

func TestNetHTTPMiddlewareEmitsEvent(t *testing.T) {
	sink, store := loza.MemorySink()
	cfg := loza.Test().WithService("test-service").WithSink(sink)
	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		loza.Enrich(ctx, loza.String("user.id", "u-1"))
		loza.Finish(ctx, "success", loza.Int("status_code", http.StatusOK))
		_, _ = io.WriteString(w, "ok")
	})

	srv := httptest.NewServer(lozahttp.Middleware(lozahttp.Config{Event: "http.request"})(mux))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/hello")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = resp.Body.Close()

	if store.Len() == 0 {
		t.Fatalf("expected at least one emitted event")
	}
}
