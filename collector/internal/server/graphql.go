package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/graphql-go/graphql"
)

type GraphQLServer struct {
	cfg    GraphQLConfig
	state  State
	ready  atomic.Bool
	mu     sync.Mutex
	server *http.Server
}

func NewGraphQLServer(cfg GraphQLConfig, state State) *GraphQLServer {
	return &GraphQLServer{
		cfg:   cfg,
		state: state,
	}
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

		result := graphql.Do(graphql.Params{
			Schema:         schema,
			RequestString:  req.Query,
			VariableValues: req.Variables,
			OperationName:  req.OperationName,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	s.server = &http.Server{
		Addr:    s.cfg.Port,
		Handler: mux,
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