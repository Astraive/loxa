package graphqlserver

import (
	internalserver "github.com/astraive/loza/collector/internal/server"
)

func New(cfg internalserver.GraphQLConfig, state internalserver.State) *internalserver.GraphQLServer {
	return internalserver.NewGraphQLServer(cfg, state)
}
