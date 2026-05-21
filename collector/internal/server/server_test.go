package server

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

type mockState struct {
	ready   bool
	healthy bool
	metrics Metrics
}

func (m *mockState) IsReady() bool   { return m.ready }
func (m *mockState) IsHealthy() bool { return m.healthy }
func (m *mockState) Ingest(ctx context.Context, events [][]byte) (int, error) {
	m.metrics.EventsAccepted += int64(len(events))
	return len(events), nil
}
func (m *mockState) GetMetrics() Metrics { return m.metrics }

func TestHTTPServerDisabled(t *testing.T) {
	cfg := HTTPConfig{Enabled: false}
	state := &mockState{ready: true, healthy: true}

	srv := NewHTTPServer(cfg, state)
	assert.Equal(t, "http", srv.Name())
	assert.False(t, srv.IsReady())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := srv.Start(ctx)
	assert.NoError(t, err)
	assert.False(t, srv.IsReady())
}

func TestHTTPServerIngest(t *testing.T) {
	cfg := HTTPConfig{
		Enabled:      true,
		Addr:         "127.0.0.1:0",
		IngestPath:   "/ingest",
		HealthPath:   "/healthz",
		ReadyPath:    "/readyz",
		MaxBodyBytes: 10 * 1024 * 1024,
		AuthEnabled:  false,
	}
	state := &mockState{ready: true, healthy: true}

	srv := NewHTTPServer(cfg, state)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	require.True(t, srv.IsReady())

	srv.Stop(context.Background())
}

func TestHTTPServerHealthReady(t *testing.T) {
	cfg := HTTPConfig{
		Enabled:    true,
		Addr:       "127.0.0.1:0",
		IngestPath: "/ingest",
		HealthPath: "/healthz",
		ReadyPath:  "/readyz",
	}
	state := &mockState{ready: true, healthy: true}

	srv := NewHTTPServer(cfg, state)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = srv.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	assert.True(t, srv.IsReady())

	srv.Stop(context.Background())
}

func TestGRPCServerDisabled(t *testing.T) {
	cfg := GRPCConfig{Enabled: false}
	state := &mockState{ready: true, healthy: true}

	srv := NewGRPCServer(cfg, state)
	assert.Equal(t, "grpc", srv.Name())
	assert.False(t, srv.IsReady())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := srv.Start(ctx)
	assert.NoError(t, err)
	assert.False(t, srv.IsReady())
}

func TestGRPCServerStart(t *testing.T) {
	cfg := GRPCConfig{
		Enabled:              true,
		Port:                 "127.0.0.1:0",
		MaxConcurrentStreams: 100,
	}
	state := &mockState{ready: true, healthy: true}

	srv := NewGRPCServer(cfg, state)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	require.True(t, srv.IsReady())

	srv.Stop(context.Background())
}

func TestGRPCCollectorServiceHealth(t *testing.T) {
	svc := &collectorSvcServer{state: &mockState{healthy: true}}

	resp, err := svc.Health(context.Background(), &CollectorStatusRequest{})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)

	svc.state = &mockState{healthy: false}
	resp, err = svc.Health(context.Background(), &CollectorStatusRequest{})
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", resp.Status)
}

func TestGRPCCollectorServiceReady(t *testing.T) {
	svc := &collectorSvcServer{state: &mockState{ready: true}}

	resp, err := svc.Ready(context.Background(), &CollectorStatusRequest{})
	require.NoError(t, err)
	assert.Equal(t, "ready", resp.Status)

	svc.state = &mockState{ready: false}
	resp, err = svc.Ready(context.Background(), &CollectorStatusRequest{})
	require.NoError(t, err)
	assert.Equal(t, "not_ready", resp.Status)
}

func TestGRPCLogIngestServicePush(t *testing.T) {
	state := &mockState{ready: true, healthy: true}
	svc := &logIngestSvcServer{state: state}

	batch := &EventBatch{
		Events: []*Event{
			{RawJson: `{"id":"1"}`},
			{RawJson: `{"id":"2"}`},
		},
	}

	resp, err := svc.Push(context.Background(), batch)
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Accepted)
	assert.Equal(t, int64(2), state.metrics.EventsAccepted)
}

func TestGRPCLogIngestServicePushEmpty(t *testing.T) {
	state := &mockState{ready: true, healthy: true}
	svc := &logIngestSvcServer{state: state}

	resp, err := svc.Push(context.Background(), &EventBatch{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.Accepted)
}

func TestGRPCLogIngestServicePushNil(t *testing.T) {
	state := &mockState{ready: true, healthy: true}
	svc := &logIngestSvcServer{state: state}

	resp, err := svc.Push(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.Accepted)
}

func TestLoxaIngestServiceIngest(t *testing.T) {
	state := &mockState{ready: true, healthy: true}
	svc := &loxaIngestSvcServer{state: state}

	resp, err := svc.Ingest(context.Background(), &EventBatch{
		Events: []*Event{
			{RawJson: `{"id":"1"}`},
			{RawJson: `{"id":"2"}`},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Accepted)
	assert.Equal(t, int64(2), state.metrics.EventsAccepted)
}

func TestLoxaIngestServiceIngestStream(t *testing.T) {
	state := &mockState{ready: true, healthy: true}
	svc := &loxaIngestSvcServer{state: state}
	stream := &mockIngestStream{
		ctx: context.Background(),
		batches: []*EventBatch{
			{Events: []*Event{{RawJson: `{"id":"1"}`}}},
			{Events: []*Event{{RawJson: `{"id":"2"}`}, {RawJson: `{"id":"3"}`}}},
		},
	}

	err := svc.IngestStream(stream)
	require.NoError(t, err)
	require.NotNil(t, stream.closed)
	assert.Equal(t, int64(3), stream.closed.Accepted)
	assert.Equal(t, int64(3), state.metrics.EventsAccepted)
}

func TestGraphQLServerDisabled(t *testing.T) {
	cfg := GraphQLConfig{Enabled: false}
	state := &mockState{ready: true, healthy: true}

	srv := NewGraphQLServer(cfg, state)
	assert.Equal(t, "graphql", srv.Name())
	assert.False(t, srv.IsReady())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := srv.Start(ctx)
	assert.NoError(t, err)
	assert.False(t, srv.IsReady())
}

func TestGraphQLServerStart(t *testing.T) {
	cfg := GraphQLConfig{
		Enabled:    true,
		Port:       "127.0.0.1:0",
		Playground: false,
	}
	state := &mockState{ready: true, healthy: true}

	srv := NewGraphQLServer(cfg, state)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = srv.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	require.True(t, srv.IsReady())

	srv.Stop(context.Background())
}

func TestDuration(t *testing.T) {
	d := NewDuration(5 * time.Second)
	assert.Equal(t, 5*time.Second, d.Duration())
}

func TestParseNDJSONEvents(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected [][]byte
		maxBytes int64
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:     "single event",
			input:    `{"id":"1"}`,
			expected: [][]byte{[]byte(`{"id":"1"}`)},
		},
		{
			name:     "multiple events",
			input:    `{"id":"1"}` + "\n" + `{"id":"2"}`,
			expected: [][]byte{[]byte(`{"id":"1"}`), []byte(`{"id":"2"}`)},
		},
		{
			name:     "empty lines skipped",
			input:    `{"id":"1"}` + "\n\n" + `{"id":"2"}`,
			expected: [][]byte{[]byte(`{"id":"1"}`), []byte(`{"id":"2"}`)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := parseNDJSONEvents(
				strings.NewReader(tt.input),
				tt.maxBytes,
			)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, events)
		})
	}
}

func TestItoa(t *testing.T) {
	assert.Equal(t, "0", itoa(0))
	assert.Equal(t, "1", itoa(1))
	assert.Equal(t, "100", itoa(100))
	assert.Equal(t, "-1", itoa(-1))
}

func TestConfigStructs(t *testing.T) {
	cfg := Config{
		HTTP: HTTPConfig{
			Enabled:           true,
			Addr:              ":9090",
			ReadHeaderTimeout: 5 * time.Second,
			MaxBodyBytes:      10 * 1024 * 1024,
		},
		GRPC: GRPCConfig{
			Enabled:              true,
			Port:                 ":9091",
			MaxConcurrentStreams: 100,
		},
		GraphQL: GraphQLConfig{
			Enabled:    true,
			Port:       ":9092",
			Playground: true,
		},
	}

	assert.True(t, cfg.HTTP.Enabled)
	assert.Equal(t, ":9090", cfg.HTTP.Addr)
	assert.True(t, cfg.GRPC.Enabled)
	assert.Equal(t, ":9091", cfg.GRPC.Port)
	assert.True(t, cfg.GraphQL.Enabled)
	assert.Equal(t, ":9092", cfg.GraphQL.Port)
}

type mockIngestStream struct {
	ctx     context.Context
	batches []*EventBatch
	closed  *PushResponse
	index   int
}

func (m *mockIngestStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockIngestStream) SendHeader(metadata.MD) error { return nil }
func (m *mockIngestStream) SetTrailer(metadata.MD)       {}
func (m *mockIngestStream) Context() context.Context     { return m.ctx }
func (m *mockIngestStream) SendMsg(interface{}) error    { return nil }
func (m *mockIngestStream) RecvMsg(interface{}) error    { return nil }

func (m *mockIngestStream) SendAndClose(resp *PushResponse) error {
	m.closed = resp
	return nil
}

func (m *mockIngestStream) Recv() (*EventBatch, error) {
	if m.index >= len(m.batches) {
		return nil, io.EOF
	}
	batch := m.batches[m.index]
	m.index++
	return batch, nil
}
