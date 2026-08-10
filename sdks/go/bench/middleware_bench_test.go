package bench

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astraive/loza/sdks/go"
	lozahttp "github.com/astraive/loza/sdks/go/src/middleware/nethttp"
)

func BenchmarkNetHTTPMiddleware(b *testing.B) {
	sink, _ := loza.MemorySink()
	_ = loza.Configure(loza.Test().WithSink(sink))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /bench", func(w http.ResponseWriter, r *http.Request) {
		loza.Finish(r.Context(), "success", loza.Int("status_code", 200))
		_, _ = io.WriteString(w, "ok")
	})

	srv := httptest.NewServer(lozahttp.Middleware(lozahttp.Config{Event: "http.request"})(mux))
	defer srv.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Get(srv.URL + "/bench")
		if err != nil {
			b.Fatalf("get: %v", err)
		}
		_ = resp.Body.Close()
	}
}
