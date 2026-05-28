package graphqlserver

import (
	internalserver "github.com/astraive/loxa/collector/internal/server"
)

func New(cfg internalserver.GraphQLConfig, state internalserver.State) *internalserver.GraphQLServer {
	return internalserver.NewGraphQLServer(cfg, state)
}
