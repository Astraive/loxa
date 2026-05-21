package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astraive/loxa-go"
	loxahttp "github.com/astraive/loxa-go/src/middleware/nethttp"
)

func TestNetHTTPMiddlewareEmitsEvent(t *testing.T) {
	sink, store := loxa.MemorySink()
	cfg := loxa.Test().WithService("test-service").WithSink(sink)
	if err := loxa.Configure(cfg); err != nil {
		t.Fatalf("configure: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		loxa.Enrich(ctx, loxa.String("user.id", "u-1"))
		loxa.Finish(ctx, "success", loxa.Int("status_code", http.StatusOK))
		_, _ = io.WriteString(w, "ok")
	})

	srv := httptest.NewServer(loxahttp.Middleware(loxahttp.Config{Event: "http.request"})(mux))
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
