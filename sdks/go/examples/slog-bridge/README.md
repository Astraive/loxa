# slog Bridge Example

```go
package main

import (
	"log/slog"
	"os"

	"github.com/Astraive/loxa/sdks/go"
	loxaslog "github.com/Astraive/loxa/sdks/go/integrations/slog"
)

func main() {
	_ = loxa.Configure(loxa.Production().WithService("payments"))
	logger := slog.New(loxaslog.Handler())
	logger.Info("payment accepted", "order_id", "ord-42", "amount", 4999)
}
```

Use this migration pattern:
1. Keep existing `slog` usage.
2. Add LOXA lifecycle events around important operations.
3. Keep line logs for diagnostics, wide events for analytics.
