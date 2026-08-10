package grpcserver

import (
	internalserver "github.com/astraive/loza/collector/internal/server"
)

func New(cfg internalserver.GRPCConfig, state internalserver.State) *internalserver.GRPCServer {
	return internalserver.NewGRPCServer(cfg, state)
}
