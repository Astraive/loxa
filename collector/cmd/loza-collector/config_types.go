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

	"github.com/astraive/loza/collector/internal/auth"
	collectorconfig "github.com/astraive/loza/collector/internal/config"
	collectorevent "github.com/astraive/loza/collector/internal/event"
	"github.com/astraive/loza/collector/internal/eventbus"
	processing "github.com/astraive/loza/collector/internal/processing"
	serverconfig "github.com/astraive/loza/collector/internal/server"
	"golang.org/x/time/rate"
)

type serverConfig struct {
	HTTP    serverconfig.HTTPConfig
	GRPC    serverconfig.GRPCConfig
	GraphQL serverconfig.GraphQLConfig
}

type collectorAuthKey struct {
	name                 string
	keyID                string
	secret               string
	kind                 auth.KeyKind
	mode                 auth.AccessMode
	roles                []auth.Role
	collector            string
	permissions          []auth.Permission
	allowedEnvs          []string
	allowedServices      []string
	allowedOrigins       []string
	allowedIPs           []string
	maxPayloadBytes      int
	maxRequestsPerMinute int
	maxEventsPerMinute   int
}

// collectorAuthGrant is the resolved runtime representation of a
// collector-bound Basic credential.
type collectorAuthGrant struct {
	name                 string
	collector            string
	keyID                string
	secret               string
	kind                 auth.KeyKind
	permissions          []auth.Permission
	allowedEnvs          []string
	allowedServices      []string
	allowedOrigins       []string
	allowedIPs           []string
	maxPayloadBytes      int
	maxRequestsPerMinute int
	maxEventsPerMinute   int
}

type collectorAuthToken struct {
	name                 string
	token                string
	mode                 auth.AccessMode
	roles                []auth.Role
	collector            string
	permissions          []auth.Permission
	allowedEnvs          []string
	allowedServices      []string
	allowedOrigins       []string
	allowedIPs           []string
	maxPayloadBytes      int
	maxRequestsPerMinute int
	maxEventsPerMinute   int
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
	authAllowLocalDevKeys   bool
	authServerSecret        string
	authCacheTTL            time.Duration
	authNegativeCacheTTL    time.Duration
	authKeys                []collectorAuthKey
	authTokens              []collectorAuthToken
	authDefaultCollector    string
	authCollectors          []string
	authGrants              []collectorAuthGrant
	apiKeyHeader            string
	apiKey                  string
	duckDBPath              string
	duckDBDriver            string
	duckDBTable             string
	lqlEnabled              bool
	lqlBinary               string
	lqlExpectedProtocol     int
	lqlExpectedCompiler     string
	lqlExpectedLanguage     string
	lqlStartupTimeout       time.Duration
	lqlCompileTimeout       time.Duration
	lqlMaxConcurrent        int
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
	rateLimitEnabled        bool
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
	mtlsAllowedCNs          []string
	mtlsAllowedDNS          []string
	mtlsAllowedEmails       []string
	allowedOrigins          []string
	trustedProxies          []string
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
	eventBusType            string
	eventBusTopic           string
	eventBusDLQTopic        string
	eventBusConsumerGroup   string
	eventBusMemoryBuffer    int
	eventBusRedisAddr       string
	eventBusRedisPassword   string
	eventBusRedisDB         int
	eventBusRedisStream     string
	eventBusRedisGroup      string
	eventBusRedisMaxLen     int64
	eventBusNATSURL         string
	eventBusNATSStream      string
	eventBusNATSSubject     string
	eventBusNATSDurable     string
	eventBusKafkaBrokers    []string
	eventBusKafkaTopic      string
	eventBusKafkaGroup      string
	eventBusKafkaAcks       string
	eventBusKafkaIdempotent bool
	eventBusKafkaMaxRetries int
	eventBusKafkaCompress   string
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
	spoolReplayCount       atomic.Int64
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
	deliveryQueue     chan spoolDelivery
	deliverySpace     chan struct{}
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
	lqlCompiler       LQLCompiler
	reliabilityCtx    context.Context
	reliabilityCancel context.CancelFunc
	retentionStop     chan struct{}
	closeOnce         sync.Once
	eventBus          eventbus.Bus
	keyStore          *memoryKeyStore
	keyCache          *auth.MemoryKeyCache
	keyRateLimiter    *auth.KeyRateLimiter
	serverSecret      []byte
}

func (c *collectorConfig) buildEventBusConfig() eventbus.Config {
	cfg := eventbus.Config{
		Type:          c.eventBusType,
		Topic:         c.eventBusTopic,
		DLQTopic:      c.eventBusDLQTopic,
		ConsumerGroup: c.eventBusConsumerGroup,
		Memory: eventbus.MemoryConfig{
			BufferSize: c.eventBusMemoryBuffer,
		},
		Redis: eventbus.RedisConfig{
			Addr:     c.eventBusRedisAddr,
			Password: c.eventBusRedisPassword,
			DB:       c.eventBusRedisDB,
			Stream:   c.eventBusRedisStream,
			Group:    c.eventBusRedisGroup,
			MaxLen:   c.eventBusRedisMaxLen,
		},
		NATS: eventbus.NATSConfig{
			URL:     c.eventBusNATSURL,
			Stream:  c.eventBusNATSStream,
			Subject: c.eventBusNATSSubject,
			Durable: c.eventBusNATSDurable,
		},
		Kafka: eventbus.KafkaConfig{
			Brokers:           c.eventBusKafkaBrokers,
			Topic:             c.eventBusKafkaTopic,
			ConsumerGroup:     c.eventBusKafkaGroup,
			Acks:              c.eventBusKafkaAcks,
			EnableIdempotence: c.eventBusKafkaIdempotent,
			MaxRetries:        c.eventBusKafkaMaxRetries,
			Compression:       c.eventBusKafkaCompress,
		},
	}
	if cfg.Topic == "" {
		cfg.Topic = "loza.events.raw"
	}
	if cfg.ConsumerGroup == "" {
		cfg.ConsumerGroup = "loza-worker"
	}
	return cfg
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
