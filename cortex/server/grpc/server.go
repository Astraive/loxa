package grpcserver

import (
	"github.com/astraive/loxa/loxa-cortex/internal/api"
	"github.com/astraive/loxa/loxa-cortex/internal/config"
	"github.com/astraive/loxa/loxa-cortex/internal/storage"
)

func New(cfg *config.Config, stor storage.Storage) *api.GRPCServer {
	return api.NewGRPCServer(cfg, stor)
}
