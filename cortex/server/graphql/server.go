package graphqlserver

import (
	"net/http"

	"github.com/astraive/loza/cortex/internal/api"
	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/storage"
)

func New(cfg *config.Config, stor storage.Storage) *api.GraphQLServer {
	return api.NewGraphQLServer(cfg, stor)
}

func Handler(cfg *config.Config, stor storage.Storage) http.Handler {
	return New(cfg, stor).Handler()
}
