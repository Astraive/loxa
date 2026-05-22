# net/http Canonical Event Example

```go
package main

import (
	"net/http"

	"github.com/astraive/loxa/sdks/go"
	loxahttp "github.com/astraive/loxa/sdks/go/middleware/nethttp"
)

func main() {
	sink, _ := loxa.MemorySink()
	_ = loxa.Configure(loxa.Production().WithService("checkout").WithSink(sink))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /checkout", func(w http.ResponseWriter, r *http.Request) {
		loxa.Enrich(r.Context(), loxa.UserID("u-1"), loxa.String("payment.provider", "stripe"))
		loxa.Finish(r.Context(), "success", loxa.Int("status_code", http.StatusOK))
		w.WriteHeader(http.StatusOK)
	})

	handler := loxahttp.Middleware(loxahttp.Config{Event: "checkout.request"})(mux)
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
