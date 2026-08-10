package websocketserver

import (
	"net/http"

	internalserver "github.com/astraive/loza/collector/internal/server"
)

func Handler(cfg internalserver.HTTPConfig, state internalserver.TailState) http.Handler {
	return internalserver.NewTailWebSocketHandler(cfg, state)
}
