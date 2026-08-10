package websocketserver

import (
	"net/http"

	"github.com/astraive/loza/cortex/internal/api"
	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/storage"
)

func Handler(cfg *config.Config, stor storage.Storage) http.Handler {
	return api.NewServer(cfg, stor).WebSocketHandler()
}
