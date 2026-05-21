package server

import (
	"context"
	"time"
)

type Duration struct {
	d time.Duration
}

func NewDuration(d time.Duration) Duration {
	return Duration{d: d}
}

func (d Duration) Duration() time.Duration {
	return d.d
}

type Config struct {
	HTTP    HTTPConfig
	GRPC    GRPCConfig
	GraphQL GraphQLConfig
}

type HTTPConfig struct {
	Enabled              bool
	Addr                 string
	ReadHeaderTimeout    time.Duration
	MaxBodyBytes         int64
	MaxHeaderBytes       int64
	IdleTimeout          time.Duration
	TLSEnabled           bool
	TLSCertFile          string
	TLSKeyFile           string
	TLSClientCAFile      string
	TLSRequireClientCert bool
	IngestPath           string
	HealthPath           string
	ReadyPath            string
	MetricsPath          string
	MetricsEnabled       bool
	AuthEnabled          bool
	AuthHeader           string
	AuthValue            string
	RateLimitEnabled     bool
	RateLimitRPS         float64
	RateLimitBurst       int
}

type GRPCConfig struct {
	Enabled               bool
	Port                  string
	MaxConnections        int
	MaxConcurrentStreams  int
	MaxRecvMsgSize        int
	MaxSendMsgSize        int
	MaxConnectionAge      Duration
	MaxConnectionAgeGrace Duration
	KeepaliveTime         Duration
	KeepaliveTimeout      Duration
	TLSCertFile           string
	TLSKeyFile            string
	TLSEnabled            bool
}

type GraphQLConfig struct {
	Enabled    bool
	Port       string
	Playground bool
	DepthLimit int
	BatchLimit int
}

type Metrics struct {
	RequestsTotal   int64
	RequestsAuthErr int64
	RequestsLimited int64
	EventsAccepted  int64
	EventsInvalid   int64
	EventsRejected  int64
	EventsDeduped   int64
}

type State interface {
	IsReady() bool
	IsHealthy() bool
	Ingest(ctx context.Context, events [][]byte) (int, error)
	GetMetrics() Metrics
}

type Server interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsReady() bool
	Addr() string
}
