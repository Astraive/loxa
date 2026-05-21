package bench

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astraive/loxa-go"
	loxahttp "github.com/astraive/loxa-go/src/middleware/nethttp"
)

func BenchmarkNetHTTPMiddleware(b *testing.B) {
	sink, _ := loxa.MemorySink()
	_ = loxa.Configure(loxa.Test().WithSink(sink))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /bench", func(w http.ResponseWriter, r *http.Request) {
		loxa.Finish(r.Context(), "success", loxa.Int("status_code", 200))
		_, _ = io.WriteString(w, "ok")
	})

	srv := httptest.NewServer(loxahttp.Middleware(loxahttp.Config{Event: "http.request"})(mux))
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
