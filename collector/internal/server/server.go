package server

import (
	"context"
	"log"
	"sync"
)

type MultiServer struct {
	cfg      Config
	state    State
	servers  []Server
	ready    bool
	mu       sync.Mutex
	started  bool
	stopped  bool
	stopOnce sync.Once
	stopCh   chan struct{}
	stopWG   sync.WaitGroup
}

func NewMultiServer(cfg Config, state State) *MultiServer {
	return &MultiServer{
		cfg:    cfg,
		state:  state,
		stopCh: make(chan struct{}),
	}
}

func (ms *MultiServer) Run(ctx context.Context) error {
	ms.mu.Lock()
	if ms.started {
		ms.mu.Unlock()
		return nil
	}
	ms.started = true

	if ms.cfg.HTTP.Enabled {
		ms.servers = append(ms.servers, NewHTTPServer(ms.cfg.HTTP, ms.state))
	}
	if ms.cfg.GRPC.Enabled {
		ms.servers = append(ms.servers, NewGRPCServer(ms.cfg.GRPC, ms.state))
	}
	if ms.cfg.GraphQL.Enabled {
		ms.servers = append(ms.servers, NewGraphQLServer(ms.cfg.GraphQL, ms.state))
	}

	if len(ms.servers) == 0 {
		ms.mu.Unlock()
		log.Println("no servers enabled, nothing to do")
		return nil
	}
	ms.mu.Unlock()

	serverCh := make(chan error, len(ms.servers))
	for _, srv := range ms.servers {
		ms.stopWG.Add(1)
		go func(s Server) {
			defer ms.stopWG.Done()
			log.Printf("starting %s server on %s", s.Name(), s.Addr())
			if err := s.Start(ctx); err != nil {
				select {
				case serverCh <- err:
				case <-ms.stopCh:
				case <-ctx.Done():
				}
			}
		}(srv)
	}

	var startErr error
	select {
	case startErr = <-serverCh:
		ms.closeStopCh()
		ms.stopWG.Wait()
		return startErr
	case <-ms.stopCh:
		return ms.shutdown(ctx)
	case <-ctx.Done():
		return ms.shutdown(ctx)
	}
}

func (ms *MultiServer) shutdown(ctx context.Context) error {
	ms.mu.Lock()
	if ms.stopped {
		ms.mu.Unlock()
		return nil
	}
	ms.stopped = true
	ms.mu.Unlock()

	ms.closeStopCh()

	errCh := make(chan error, len(ms.servers))
	for _, srv := range ms.servers {
		ms.stopWG.Add(1)
		go func(s Server) {
			defer ms.stopWG.Done()
			log.Printf("stopping %s server", s.Name())
			if err := s.Stop(ctx); err != nil {
				errCh <- err
			}
		}(srv)
	}

	ms.stopWG.Wait()

	close(errCh)
	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (ms *MultiServer) closeStopCh() {
	ms.stopOnce.Do(func() {
		close(ms.stopCh)
	})
}

func (ms *MultiServer) IsReady() bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, srv := range ms.servers {
		if !srv.IsReady() {
			return false
		}
	}
	return true
}
