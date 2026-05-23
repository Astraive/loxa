package main

import (
	"context"
	"database/sql"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	collectorconfig "github.com/astraive/loxa-collector/internal/config"
	collectorevent "github.com/astraive/loxa-collector/internal/event"
	processing "github.com/astraive/loxa-collector/internal/processing"
	serverconfig "github.com/astraive/loxa-collector/internal/server"
	"golang.org/x/time/rate"
)

type serverConfig struct {
	HTTP    serverconfig.HTTPConfig
	GRPC    serverconfig.GRPCConfig
	GraphQL serverconfig.GraphQLConfig
}

type collectorConfig struct {
	configFile              string
	configArgs              []string
	readHeaderTimeout       time.Duration
	addr                    string
	shutdownTimeout         time.Duration
	maxBodyBytes            int64
	maxEventsPerRequest     int
	serverConfig            serverConfig
	authEnabled             bool
	apiKeyHeader            string
	apiKey                  string
	rateLimitEnabled        bool
	duckDBPath              string
	duckDBDriver            string
	duckDBTable             string
	duckDBRawColumn         string
	duckDBStoreRaw          bool
	duckDBCheckpointOnStop  bool
	duckDBMaxOpenConns      int
	duckDBMaxIdleConns      int
	duckDBBatchSize         int
	duckDBFlushInterval     time.Duration
	duckDBWriterLoop        bool
	duckDBWriterQueueSize   int
	duckDBUseAppender       bool
	duckDBWriteTimeout      time.Duration
	duckDBRetryAttempts     int
	duckDBRetryBackoff      time.Duration
	duckDBCheckpointIntvl   time.Duration
	duckDBExportEnabled     bool
	duckDBExportFormat      string
	duckDBExportInterval    time.Duration
	duckDBExportPath        string
	duckDBSchema            map[string]string
	duckDBColumnTypes       map[string]string
	ingestPath              string
	healthPath              string
	readyPath               string
	metricsPath             string
	storagePrimary          string
	storageEncryptionKey    string
	rateLimitRPS            float64
	rateLimitBurst          int
	loggingLevel            string
	loggingFormat           string
	metricsPrometheus       bool
	reliabilityMode         string
	spoolDir                string
	spoolFile               string
	maxSpoolBytes           int64
	spoolFsync              bool
	deliveryQueueSize       int
	maxInflightRequests     int
	maxInflightEvents       int
	maxQueueBytes           int64
	maxEventBytes           int64
	maxAttrCount            int
	maxAttrDepth            int
	maxStringLength         int
	identityMode            string
	authIdentityWins        bool
	allowPayloadIdentity    bool
	boundServiceName        string
	boundServiceVersion     string
	boundEnvironment        string
	boundRegion             string
	boundTenantID           string
	boundWorkspaceID        string
	boundOrganizationID     string
	privacyMode             string
	collectorRedaction      bool
	emergencyRedaction      bool
	privacyAllowlist        []string
	privacyBlocklist        []string
	secretScan              bool
	rightToDeleteEnabled    bool
	receiverRegistry        []string
	processorRegistry       []string
	exporterRegistry        []string
	extensionRegistry       []string
	queueDir                string
	queueBatchSize          int
	queueBatchTimeout       time.Duration
	queueFlushInterval      time.Duration
	queueCircuitThreshold   int
	queueCircuitTimeout     time.Duration
	retryEnabled            bool
	retryMaxAttempts        int
	retryInitialBackoff     time.Duration
	retryMaxBackoff         time.Duration
	retryJitter             bool
	dlqEnabled              bool
	dlqPath                 string
	fanoutOutputs           []collectorFanoutOutput
	deliveryPolicy          string
	fallbackEnabled         bool
	fallbackOnPrimaryFail   bool
	fallbackOnSecondaryFail bool
	fallbackOnPolicyFail    bool
	dlqOnPrimaryFail        bool
	dlqOnSecondaryFail      bool
	dlqOnFallbackFail       bool
	dlqOnPolicyFail         bool
	dedupeEnabled           bool
	dedupeKey               string
	dedupeWindow            time.Duration
	dedupeBackend           string
	dedupeRedisAddr         string
	dedupeRedisPassword     string
	dedupeRedisDB           int
	dedupeRedisPrefix       string
	kafkaBrokers            []string
	kafkaTopic              string
	kafkaAcks               string
	kafkaRequestTimeout     time.Duration
	kafkaIdempotence        bool
	kafkaMaxRetries         int
	kafkaRetryBackoff       time.Duration
	workerConsumerGroup     string
	workerPollTimeout       time.Duration
	schemaMode              string
	schemaSchemaVersion     string
	schemaEventVersion      string
	schemaQuarantinePath    string
	schemaRegistryFile      string
	schemaRegistry          []collectorconfig.SchemaRegistryEntry
	retentionEnabled        bool
	retentionDays           int
	retentionMaxSize        int64
	cortexBridgeEnabled     bool
	cortexBridgeMode        string
	cortexBridgeEndpoint    string
	cortexBridgeInsecure    bool
	cortexBridgeTimeout     time.Duration
	cortexBridgeBatchSize   int
	cortexBridgeFlushIntvl  time.Duration
	cortexBridgeQueueSize   int
	cortexBridgeHeader      string
	cortexBridgeAPIKey      string
	cortexSchemaEnabled     bool
}

type collectorMetrics struct {
	requestsTotal          atomic.Int64
	requestsAuthErr        atomic.Int64
	requestsLimited        atomic.Int64
	requestsThrottled      atomic.Int64
	eventsAccepted         atomic.Int64
	eventsInvalid          atomic.Int64
	eventsRejected         atomic.Int64
	eventsDeduped          atomic.Int64
	sinkWriteErrors        atomic.Int64
	spoolBytes             atomic.Int64
	queueBytes             atomic.Int64
	inflightRequests       atomic.Int64
	inflightEvents         atomic.Int64
	spoolReplayCount       int64
	cortexBridgeFlushes    atomic.Int64
	cortexBridgeEvents     atomic.Int64
	cortexBridgeErrors     atomic.Int64
	cortexBridgeQueueDepth atomic.Int64
}

type collectorState struct {
	cfg               collectorConfig
	ingestSink        collectorevent.Sink
	hybridQueueSink   collectorevent.Sink
	secondarySinks    []namedSink
	fallbackSink      *namedSink
	ready             atomic.Bool
	sinkHealthy       atomic.Bool
	spoolHealthy      atomic.Bool
	diskHealthy       atomic.Bool
	rateLimiter       *rate.Limiter
	metrics           collectorMetrics
	rng               *rand.Rand
	spoolFile         *os.File
	spoolPosFile      string
	spoolBadFile      string
	spoolProcessedPos int64
	spoolMu           sync.Mutex
	deliveryQueue     chan []byte
	deliveryWG        sync.WaitGroup
	metricsInit       sync.Once
	metricsHTTP       http.Handler
	dedupeMu          sync.Mutex
	dedupeSeenAt      map[string]time.Time
	dedupeStore       dedupeStore
	tailMu            sync.Mutex
	tailSubscribers   map[chan []byte]struct{}
	processorMu       sync.RWMutex
	processor         *processing.Processor
	cortexBridge      *cortexBridgeClient
	queryDB           *sql.DB
	reliabilityCtx    context.Context
	reliabilityCancel context.CancelFunc
	retentionStop     chan struct{}
	closeOnce         sync.Once
}

func (s *collectorState) GetMetrics() serverconfig.Metrics {
	return serverconfig.Metrics{
		RequestsTotal:   s.metrics.requestsTotal.Load(),
		RequestsAuthErr: s.metrics.requestsAuthErr.Load(),
		RequestsLimited: s.metrics.requestsLimited.Load(),
		EventsAccepted:  s.metrics.eventsAccepted.Load(),
		EventsInvalid:   s.metrics.eventsInvalid.Load(),
		EventsRejected:  s.metrics.eventsRejected.Load(),
		EventsDeduped:   s.metrics.eventsDeduped.Load(),
	}
}

func (s *collectorState) IsHealthy() bool {
	return s.sinkHealthy.Load() && s.diskHealthy.Load()
}

func (s *collectorState) IsReady() bool {
	return s.isReady()
}

func (s *collectorState) Ingest(ctx context.Context, events [][]byte) (int, error) {
	// Delegate to the existing handler logic via handleIngestBatch
	return s.handleIngestBatch(ctx, events)
}
