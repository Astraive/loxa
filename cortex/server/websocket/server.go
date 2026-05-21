package websocketserver

import (
	"net/http"

	"github.com/astraive/loxa/loxa-cortex/internal/api"
	"github.com/astraive/loxa/loxa-cortex/internal/config"
	"github.com/astraive/loxa/loxa-cortex/internal/storage"
)

func Handler(cfg *config.Config, stor storage.Storage) http.Handler {
	return api.NewServer(cfg, stor).WebSocketHandler()
}
