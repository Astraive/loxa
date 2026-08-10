# net/http Canonical Event Example

```go
package main

import (
	"net/http"

	"github.com/astraive/loza/sdks/go"
	lozahttp "github.com/astraive/loza/sdks/go/middleware/nethttp"
)

func main() {
	sink, _ := loza.MemorySink()
	_ = loza.Configure(loza.Production().WithService("checkout").WithSink(sink))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /checkout", func(w http.ResponseWriter, r *http.Request) {
		loza.Enrich(r.Context(), loza.UserID("u-1"), loza.String("payment.provider", "stripe"))
		loza.Finish(r.Context(), "success", loza.Int("status_code", http.StatusOK))
		w.WriteHeader(http.StatusOK)
	})

	handler := lozahttp.Middleware(lozahttp.Config{Event: "checkout.request"})(mux)
	_ = http.ListenAndServe(":8080", handler)
}
```

Sample event payload:

```json
{
  "timestamp": "2026-05-11T10:10:42Z",
  "event": "checkout.request",
  "service": "checkout",
  "method": "POST",
  "path": "/checkout",
  "route": "/checkout",
  "status_code": 200,
  "duration_ms": 42,
  "user": { "id": "u-1" },
  "payment": { "provider": "stripe" },
  "outcome": "success"
}
```
