package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/astraive/loza/collector/internal/auth"
	"github.com/graphql-go/graphql"
)

const maxGraphQLDepth = 10

type GraphQLServer struct {
	cfg               GraphQLConfig
	state             State
	ready             atomic.Bool
	server            *http.Server
	authEnabled       bool
	allowLocalDevKeys bool
	keyStore          auth.KeyStore
	keyCache          *auth.MemoryKeyCache
	serverSecret      []byte
}

func NewGraphQLServer(cfg GraphQLConfig, state State) *GraphQLServer {
	return &GraphQLServer{
		cfg:   cfg,
		state: state,
	}
}

// WithAuth configures API key authentication for the GraphQL server.
// When set, all requests to the GraphQL endpoint are authenticated.
func (s *GraphQLServer) WithAuth(store auth.KeyStore, cache *auth.MemoryKeyCache, serverSecret []byte) *GraphQLServer {
	s.authEnabled = true
	s.keyStore = store
	s.keyCache = cache
	s.serverSecret = serverSecret
	return s
}

// WithAllowLocalDevKeys enables lx_local_dev_* key acceptance on this server.
func (s *GraphQLServer) WithAllowLocalDevKeys(v bool) *GraphQLServer {
	s.allowLocalDevKeys = v
	return s
}

func (s *GraphQLServer) Name() string { return "graphql" }

func (s *GraphQLServer) Addr() string { return s.cfg.Port }

func (s *GraphQLServer) IsReady() bool { return s.ready.Load() }

func (s *GraphQLServer) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		return nil
	}

	metricsType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Metrics",
		Fields: graphql.Fields{
			"requestsTotal":   &graphql.Field{Type: graphql.Int},
			"requestsAuthErr": &graphql.Field{Type: graphql.Int},
			"requestsLimited": &graphql.Field{Type: graphql.Int},
			"eventsAccepted":  &graphql.Field{Type: graphql.Int},
			"eventsInvalid":   &graphql.Field{Type: graphql.Int},
			"eventsRejected":  &graphql.Field{Type: graphql.Int},
			"eventsDeduped":   &graphql.Field{Type: graphql.Int},
		},
	})

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"health": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return s.state.IsHealthy(), nil
				},
			},
			"ready": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return s.state.IsReady(), nil
				},
			},
			"metrics": &graphql.Field{
				Type: metricsType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					m := s.state.GetMetrics()
					return map[string]interface{}{
						"requestsTotal":   m.RequestsTotal,
						"requestsAuthErr": m.RequestsAuthErr,
						"requestsLimited": m.RequestsLimited,
						"eventsAccepted":  m.EventsAccepted,
						"eventsInvalid":   m.EventsInvalid,
						"eventsRejected":  m.EventsRejected,
						"eventsDeduped":   m.EventsDeduped,
					}, nil
				},
			},
		},
	})

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query         string                 `json:"query"`
			OperationName string                 `json:"operationName"`
			Variables     map[string]interface{} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if isIntrospectionQuery(req.Query) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "introspection_disabled"})
			return
		}

		if queryDepth(req.Query) > maxGraphQLDepth {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "query_exceeds_maximum_depth"})
			return
		}

		result := graphql.Do(graphql.Params{
			Schema:         schema,
			RequestString:  req.Query,
			VariableValues: req.Variables,
			OperationName:  req.OperationName,
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	var handler http.Handler = mux
	if s.authEnabled {
		authMW := auth.Middleware(s.keyStore, s.keyCache, s.serverSecret, auth.WithAllowLocalDevKeys(s.allowLocalDevKeys))
		handler = authMW(handler)
	}

	s.server = &http.Server{
		Addr:    s.cfg.Port,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	lis, err := net.Listen("tcp", s.cfg.Port)
	if err != nil {
		return err
	}

	s.ready.Store(true)
	return s.server.Serve(lis)
}

func (s *GraphQLServer) Stop(ctx context.Context) error {
	s.ready.Store(false)
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// isIntrospectionQuery checks if the query attempts GraphQL introspection.
func isIntrospectionQuery(query string) bool {
	return containsWord(query, "__schema") ||
		containsWord(query, "__type") ||
		containsWord(query, "__typename")
}

func containsWord(s, word string) bool {
	for i := 0; i <= len(s)-len(word); i++ {
		if i > 0 && isAlpha(rune(s[i-1])) {
			continue
		}
		if i+len(word) <= len(s) && !isAlpha(rune(s[i+len(word)])) {
			if s[i:i+len(word)] == word {
				return true
			}
		}
	}
	return false
}

func isAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
}

// queryDepth counts the maximum nesting depth of curly braces in a GraphQL query.
func queryDepth(query string) int {
	maxDepth := 0
	depth := 0
	for _, c := range query {
		switch c {
		case '{':
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
		case '}':
			depth--
		}
	}
	return maxDepth
}