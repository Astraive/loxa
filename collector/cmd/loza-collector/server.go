package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/astraive/loza/collector/internal/auth"
	"github.com/astraive/loza/collector/internal/database"
	collectorevent "github.com/astraive/loza/collector/internal/event"
	"github.com/astraive/loza/collector/internal/eventbus"
	processing "github.com/astraive/loza/collector/internal/processing"
	serverruntime "github.com/astraive/loza/collector/internal/server"
	"github.com/astraive/loza/collector/internal/sinks/duckdb"
	kafkasink "github.com/astraive/loza/collector/internal/sinks/kafka"
	storagepath "github.com/astraive/loza/collector/internal/storage"
	publichttp "github.com/astraive/loza/collector/server/http"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

func runCollector(cfg collectorConfig) error {
	cfg = withOwnershipProjections(cfg)

	var (
		db               *sql.DB
		err              error
		sink             collectorevent.Sink
		hybridQueueSink  collectorevent.Sink
		secondarySinks   []namedSink
		fallbackSink     *namedSink
		fanoutDBs        []*sql.DB
		schedulersStop   chan struct{}
		schedWG          sync.WaitGroup
		errCh            = make(chan error, 4)
		bus              eventbus.Bus
		namedConnections map[string]database.Connection
		namedMetadata    map[string]database.Metadata
	)

	if cfg.authEnabled && strings.TrimSpace(cfg.authServerSecret) == "" {
		return errors.New("auth.enabled requires a resolved auth.server_secret")
	}
	namedConnections, namedMetadata, err = openNamedDatabaseConnections(context.Background(), cfg)
	if err != nil {
		return err
	}
	defer func() {
		for _, connection := range namedConnections {
			_ = connection.Close(context.Background())
		}
	}()

	if cfg.reliabilityMode == "queue" {
		// Eventbus-based queue mode (pluggable: memory, redis, nats, kafka)
		busType := cfg.eventBusType
		if busType == "" {
			busType = "kafka" // backward compat: if no eventbus type, use kafka directly
		}

		if busType == "kafka" && len(cfg.eventBusKafkaBrokers) == 0 && len(cfg.kafkaBrokers) > 0 {
			// Backward compat: legacy kafka config with no eventbus config
			sink, err = kafkasink.New(kafkasink.Config{
				Brokers:           cfg.kafkaBrokers,
				Topic:             cfg.kafkaTopic,
				Acks:              cfg.kafkaAcks,
				RequestTimeout:    cfg.kafkaRequestTimeout,
				EnableIdempotence: cfg.kafkaIdempotence,
				MaxRetries:        cfg.kafkaMaxRetries,
				RetryBackoff:      cfg.kafkaRetryBackoff,
			})
			if err != nil {
				return fmt.Errorf("failed to create kafka sink: %w", err)
			}
			logJSON("info", "kafka_sink_initialized", map[string]any{
				"brokers":     cfg.kafkaBrokers,
				"topic":       cfg.kafkaTopic,
				"reliability": "at-least-once (configure broker/producer for exactly-once)",
			})
		} else {
			// New eventbus path
			busCfg := cfg.buildEventBusConfig()
			bus, err = eventbus.New(context.Background(), busCfg)
			if err != nil {
				return fmt.Errorf("failed to create eventbus: %w", err)
			}
			topic := busCfg.Topic
			if topic == "" {
				topic = "loza.events.raw"
			}
			sink = eventbus.NewSinkAdapter(bus, topic)
			logJSON("info", "eventbus_initialized", map[string]any{
				"type":        busType,
				"topic":       topic,
				"reliability": "at-least-once",
			})
		}
	} else {
		db, err = sql.Open(cfg.duckDBDriver, cfg.duckDBPath)
		if err != nil {
			return fmt.Errorf("failed to open duckdb: %w", err)
		}
		defer db.Close()
		db.SetMaxOpenConns(cfg.duckDBMaxOpenConns)
		db.SetMaxIdleConns(cfg.duckDBMaxIdleConns)

		if err := ensureSchema(db, cfg); err != nil {
			return fmt.Errorf("failed to initialize schema: %w", err)
		}

		if cfg.cortexSchemaEnabled {
			if err := ensureCortexSchema(db); err != nil {
				return fmt.Errorf("failed to initialize cortex schema: %w", err)
			}
		}

		sink, err = duckdb.New(duckdb.Config{
			DB:              db,
			Driver:          cfg.duckDBDriver,
			Table:           cfg.duckDBTable,
			StoreRaw:        cfg.duckDBStoreRaw,
			RawColumn:       cfg.duckDBRawColumn,
			Schema:          cfg.duckDBSchema,
			BatchSize:       cfg.duckDBBatchSize,
			FlushInterval:   cfg.duckDBFlushInterval,
			WriterLoop:      cfg.duckDBWriterLoop,
			WriterQueueSize: cfg.duckDBWriterQueueSize,
			EncryptRaw:      true,
			EncryptKey:      cfg.storageEncryptionKey,
		})
		if err != nil {
			return fmt.Errorf("failed to create duckdb sink: %w", err)
		}
		if cfg.reliabilityMode == "hybrid" {
			hybridQueueSink, err = kafkasink.New(kafkasink.Config{
				Brokers:           cfg.kafkaBrokers,
				Topic:             cfg.kafkaTopic,
				Acks:              cfg.kafkaAcks,
				RequestTimeout:    cfg.kafkaRequestTimeout,
				EnableIdempotence: cfg.kafkaIdempotence,
				MaxRetries:        cfg.kafkaMaxRetries,
				RetryBackoff:      cfg.kafkaRetryBackoff,
			})
			if err != nil {
				return fmt.Errorf("failed to create hybrid kafka sink: %w", err)
			}
		}
		secondarySinks, fallbackSink, fanoutDBs, err = createFanoutSinks(cfg)
		if err != nil {
			return err
		}
		defer func() {
			for _, fanoutDB := range fanoutDBs {
				_ = fanoutDB.Close()
			}
		}()

		schedulersStop = make(chan struct{})
		if cfg.duckDBCheckpointIntvl > 0 {
			schedWG.Add(1)
			go runPeriodicCheckpoint(db, cfg.duckDBCheckpointIntvl, schedulersStop, &schedWG)
		}
		if cfg.duckDBExportEnabled {
			schedWG.Add(1)
			go runPeriodicExport(db, cfg, schedulersStop, &schedWG)
		}
	}

	rateLimiter := rate.NewLimiter(rate.Inf, 0)
	if cfg.rateLimitEnabled {
		rateLimiter = rate.NewLimiter(rate.Limit(cfg.rateLimitRPS), cfg.rateLimitBurst)
	}
	dedupeStore, err := newRedisDedupeStore(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize dedupe store: %w", err)
	}

	var queryConnection database.Connection
	if cfg.storageConnection != "" {
		queryConnection = namedConnections[cfg.storageConnection]
		if queryConnection == nil {
			return fmt.Errorf("storage.connection %q is not enabled", cfg.storageConnection)
		}
	} else if db != nil {
		queryConnection = &database.SQL{DB: db, Info: database.Metadata{
			Name: "primary", Backend: "duckdb", Path: cfg.duckDBPath,
			Table: cfg.duckDBTable, Enabled: true, Primary: true,
			Capabilities: []string{"query", "health", "write"},
		}}
	}
	state := &collectorState{
		cfg:                 cfg,
		startedAt:           time.Now(),
		ingestSink:          sink,
		hybridQueueSink:     hybridQueueSink,
		secondarySinks:      secondarySinks,
		fallbackSink:        fallbackSink,
		rateLimiter:         rateLimiter,
		rng:                 rand.New(rand.NewSource(time.Now().UnixNano())),
		dedupeSeenAt:        make(map[string]time.Time),
		dedupeStore:         dedupeStore,
		queryDB:             db,
		queryConnection:     queryConnection,
		databaseConnections: namedConnections,
		databaseMetadata:    namedMetadata,
		eventBus:            bus,
	}
	if err := initializeAuthState(state); err != nil {
		return err
	}
	if cfg.lqlEnabled {
		compiler, err := newLQLStdioCompiler(context.Background(), cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize lql compiler: %w", err)
		}
		state.lqlCompiler = compiler
		defer func() { _ = compiler.Close(context.Background()) }()
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)

	if err := state.initReliability(); err != nil {
		return err
	}
	defer state.closeReliability()
	if state.cortexBridge, err = newCortexBridgeClient(cfg, &state.metrics); err != nil {
		return fmt.Errorf("failed to initialize cortex bridge: %w", err)
	}
	defer func() {
		if state.cortexBridge != nil {
			_ = state.cortexBridge.Close()
		}
	}()

	state.initRetention()

	state.processor, err = processing.New(processing.Config{
		DeliveryPolicy:          cfg.deliveryPolicy,
		RetryEnabled:            cfg.retryEnabled,
		RetryMaxAttempts:        cfg.retryMaxAttempts,
		RetryInitialBackoff:     cfg.retryInitialBackoff,
		RetryMaxBackoff:         cfg.retryMaxBackoff,
		RetryJitter:             cfg.retryJitter,
		FallbackEnabled:         cfg.fallbackEnabled,
		FallbackOnPrimaryFail:   cfg.fallbackOnPrimaryFail,
		FallbackOnSecondaryFail: cfg.fallbackOnSecondaryFail,
		FallbackOnPolicyFail:    cfg.fallbackOnPolicyFail,
		DLQEnabled:              cfg.dlqEnabled,
		DLQPath:                 cfg.dlqPath,
		DLQOnPrimaryFail:        cfg.dlqOnPrimaryFail,
		DLQOnSecondaryFail:      cfg.dlqOnSecondaryFail,
		DLQOnFallbackFail:       state.cfg.dlqOnFallbackFail,
		DLQOnPolicyFail:         state.cfg.dlqOnPolicyFail,
		DedupeEnabled:           state.dedupeEnabled(),
		DedupeKey:               cfg.dedupeKey,
		DedupeWindow:            cfg.dedupeWindow,
		DedupeBackend:           cfg.dedupeBackend,
		DedupeRedisAddr:         cfg.dedupeRedisAddr,
		DedupeRedisPassword:     cfg.dedupeRedisPassword,
		DedupeRedisDB:           cfg.dedupeRedisDB,
		DedupeRedisPrefix:       cfg.dedupeRedisPrefix,
		OnDiskFull: func() {
			state.diskHealthy.Store(false)
		},
		OnDLQWriteFail: func(n int64) {
			state.metrics.sinkWriteErrors.Add(n)
		},
		OnSchemaWarn: func(err error) {
			logJSON("warn", "schema_validation_warning", map[string]any{"error": err.Error()})
		},
		Schema: processing.SchemaConfig{
			Mode:           schemaModeForProcessor(state),
			SchemaVersion:  state.cfg.schemaSchemaVersion,
			EventVersion:   state.cfg.schemaEventVersion,
			QuarantinePath: state.cfg.schemaQuarantinePath,
			Registry:       state.convertSchemaRegistry(),
		},
	}, sink, secondarySinks, fallbackSink, state.rng)
	if err != nil {
		return err
	}

	mux := buildMux(state)
	muxWithSecurity := withSecurityHeaders(mux)
	muxWithCompression := withResponseCompression(muxWithSecurity)

	server := &http.Server{
		Addr:              cfg.addr,
		Handler:           muxWithCompression,
		ReadHeaderTimeout: cfg.readHeaderTimeout,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       cfg.serverConfig.HTTP.IdleTimeout,
	}
	if err := configureHTTPServerTLS(server, cfg); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parse trusted proxy CIDRs once for all servers
	var trustedCIDRs []*net.IPNet
	for _, cidr := range cfg.trustedProxies {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			log.Fatal().Err(err).Msgf("invalid trusted_proxies CIDR %q", cidr)
		}
		trustedCIDRs = append(trustedCIDRs, ipNet)
	}

	auxServers := make([]serverruntime.Server, 0, 2)
	if cfg.serverConfig.GRPC.Enabled {
		grpcSrv := serverruntime.NewGRPCServer(cfg.serverConfig.GRPC, state)
		if cfg.authEnabled {
			grpcSrv.WithAuth(state.keyStore, state.keyCache, state.serverSecret)
			grpcSrv.WithAllowLocalDevKeys(cfg.authAllowLocalDevKeys)
			grpcSrv.WithTrustedProxies(trustedCIDRs)
		}
		auxServers = append(auxServers, grpcSrv)
	}
	if cfg.serverConfig.GraphQL.Enabled {
		graphqlSrv := serverruntime.NewGraphQLServer(cfg.serverConfig.GraphQL, state)
		if cfg.authEnabled {
			graphqlSrv.WithAuth(state.keyStore, state.keyCache, state.serverSecret)
			graphqlSrv.WithAllowLocalDevKeys(cfg.authAllowLocalDevKeys)
		}
		auxServers = append(auxServers, graphqlSrv)
	}
	for _, srv := range auxServers {
		srv := srv
		go func() {
			if err := srv.Start(ctx); err != nil {
				select {
				case errCh <- fmt.Errorf("%s server: %w", srv.Name(), err):
				default:
				}
			}
		}()
	}

	go func() {
		var err error
		if cfg.serverConfig.HTTP.TLSEnabled {
			err = server.ListenAndServeTLS(cfg.serverConfig.HTTP.TLSCertFile, cfg.serverConfig.HTTP.TLSKeyFile)
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case errCh <- fmt.Errorf("listen: %w", err):
			default:
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	for {
		select {
		case startErr := <-errCh:
			shutdownCollector(server, auxServers, state, db, cfg, schedulersStop, &schedWG)
			return startErr
		case sig := <-quit:
			if sig == syscall.SIGHUP {
				if err := reloadCollectorState(state); err != nil {
					logJSON("error", "collector_reload_failed", map[string]any{"error": err.Error()})
				} else {
					logJSON("info", "collector_reload_complete", map[string]any{"config": state.cfg.configFile})
				}
				continue
			}
			shutdownCollector(server, auxServers, state, db, cfg, schedulersStop, &schedWG)
			return nil
		}
	}
}

func shutdownCollector(server *http.Server, auxServers []serverruntime.Server, state *collectorState, db *sql.DB, cfg collectorConfig, schedulersStop chan struct{}, schedWG *sync.WaitGroup) {
	logJSON("info", "collector_shutdown_begin", nil)
	state.ready.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logJSON("error", "collector_http_shutdown_failed", map[string]any{"error": err.Error()})
	}
	for _, srv := range auxServers {
		if err := srv.Stop(ctx); err != nil {
			logJSON("error", "collector_aux_shutdown_failed", map[string]any{"server": srv.Name(), "error": err.Error()})
		}
	}
	state.closeReliability()
	for _, sink := range state.sinksForShutdown() {
		if err := sink.Sink.Flush(ctx); err != nil {
			logJSON("error", "collector_sink_flush_failed", map[string]any{"sink": sink.Name, "error": err.Error()})
		}
		if err := sink.Sink.Close(ctx); err != nil {
			logJSON("error", "collector_sink_close_failed", map[string]any{"sink": sink.Name, "error": err.Error()})
		}
	}
	if state.dedupeStore != nil {
		if err := state.dedupeStore.Close(); err != nil {
			logJSON("error", "collector_dedupe_store_close_failed", map[string]any{"error": err.Error()})
		}
	}
	if state.keyRateLimiter != nil {
		state.keyRateLimiter.Close()
	}
	if state.keyCache != nil {
		state.keyCache.Close()
	}
	if state.cortexBridge != nil {
		if err := state.cortexBridge.Close(); err != nil {
			logJSON("error", "collector_cortex_bridge_close_failed", map[string]any{"error": err.Error()})
		}
	}
	if state.lqlCompiler != nil {
		if err := state.lqlCompiler.Close(ctx); err != nil {
			logJSON("error", "collector_lql_compiler_close_failed", map[string]any{"error": err.Error()})
		}
	}
	if db != nil && cfg.duckDBCheckpointOnStop {
		if _, err := db.ExecContext(ctx, "CHECKPOINT"); err != nil {
			logJSON("error", "collector_duckdb_checkpoint_failed", map[string]any{"error": err.Error()})
		}
	}
	if schedulersStop != nil {
		close(schedulersStop)
		schedWG.Wait()
	}
	if state.retentionStop != nil {
		close(state.retentionStop)
	}
	if state.eventBus != nil {
		_ = state.eventBus.Close(context.Background())
	}
	logJSON("info", "collector_shutdown_complete", nil)
}

func reloadCollectorState(state *collectorState) error {
	if len(state.cfg.configArgs) == 0 {
		return fmt.Errorf("collector reload requires original startup arguments")
	}
	next, err := loadCollectorConfigFromArgs(state.cfg.configArgs)
	if err != nil {
		return err
	}
	// Keep transport/storage topology stable during hot reload; only update mutable runtime policy.
	next.addr = state.cfg.addr
	next.serverConfig = state.cfg.serverConfig
	next.duckDBPath = state.cfg.duckDBPath
	next.duckDBDriver = state.cfg.duckDBDriver
	next.duckDBTable = state.cfg.duckDBTable
	next.duckDBRawColumn = state.cfg.duckDBRawColumn
	next.duckDBStoreRaw = state.cfg.duckDBStoreRaw
	next.reliabilityMode = state.cfg.reliabilityMode
	next.spoolDir = state.cfg.spoolDir
	next.spoolFile = state.cfg.spoolFile
	next.queueDir = state.cfg.queueDir
	next.kafkaBrokers = append([]string(nil), state.cfg.kafkaBrokers...)
	next.kafkaTopic = state.cfg.kafkaTopic

	state.cfg = next
	if state.rateLimiter != nil {
		if next.rateLimitEnabled {
			state.rateLimiter.SetLimit(rate.Limit(next.rateLimitRPS))
			state.rateLimiter.SetBurst(next.rateLimitBurst)
		} else {
			state.rateLimiter.SetLimit(rate.Inf)
			state.rateLimiter.SetBurst(0)
		}
	}
	state.processorMu.Lock()
	if state.processor != nil {
		_ = state.processor.Close()
		state.processor = nil
	}
	state.processorMu.Unlock()
	return nil
}

func buildMux(state *collectorState) *http.ServeMux {
	tailWSHandler := serverruntime.NewTailWebSocketHandler(state.cfg.serverConfig.HTTP, state)

	var protector publichttp.RouteProtector
	var authMW func(http.Handler) http.Handler
	collectorProtector := func(next http.Handler, permission string, resolve publichttp.CollectorResolver, mode publichttp.CollectorRouteMode) http.Handler {
		guard := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			collector, ok := resolve(r)
			if !ok {
				http.NotFound(w, r)
				return
			}
			environment := strings.TrimSpace(r.Header.Get("X-Loza-Env"))
			if state.cfg.authEnabled && mode == publichttp.CanonicalCollectorRoute {
				ac := auth.GetAuthContext(r.Context())
				if ac == nil || !ac.AuthorizesCollector(collector, environment, auth.Permission(permission)) {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			}
			ctx := publichttp.WithAuthorizedCollector(r.Context(), collector, environment)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
		if state.cfg.authEnabled && mode == publichttp.CanonicalCollectorRoute {
			return authMW(guard)
		}
		return guard
	}
	if state.cfg.authEnabled {
		if state.keyStore == nil || state.keyCache == nil || len(state.serverSecret) == 0 {
			if err := initializeAuthState(state); err != nil {
				log.Fatal().Err(err).Msg("invalid collector authentication configuration")
			}
		}
		var trustedCIDRs []*net.IPNet
		for _, cidr := range state.cfg.trustedProxies {
			_, ipNet, err := net.ParseCIDR(cidr)
			if err != nil {
				log.Fatal().Err(err).Msgf("invalid trusted_proxies CIDR %q", cidr)
			}
			trustedCIDRs = append(trustedCIDRs, ipNet)
		}

		authMW = auth.Middleware(state.keyStore, state.keyCache, state.serverSecret,
			auth.WithAllowLocalDevKeys(state.cfg.authAllowLocalDevKeys),
			auth.WithTrustedProxies(trustedCIDRs),
			auth.WithRateLimiter(state.keyRateLimiter),
		)
		protector = func(next http.Handler, perm string) http.Handler {
			return authMW(auth.RequirePermission(next, auth.Permission(perm)))
		}
	}

	return publichttp.BuildMux(
		state.cfg.ingestPath,
		state.cfg.healthPath,
		state.cfg.readyPath,
		state.cfg.metricsPath,
		state.cfg.metricsPrometheus,
		state.metricsHandler(),
		tailWSHandler,
		state,
		protector,
		collectorProtector,
		state.cfg.authDefaultCollector,
	)
}

func initializeAuthState(state *collectorState) error {
	if !state.cfg.authEnabled {
		return nil
	}
	if strings.TrimSpace(state.cfg.authServerSecret) == "" {
		return errors.New("auth.enabled requires a resolved auth.server_secret")
	}
	if len(state.cfg.authKeys) == 0 && len(state.cfg.authTokens) == 0 && strings.TrimSpace(state.cfg.apiKey) == "" {
		return errors.New("auth.enabled requires at least one configured key, token, or auth.value")
	}
	if state.cfg.authCacheTTL <= 0 || state.cfg.authNegativeCacheTTL <= 0 {
		return errors.New("auth cache TTLs must be positive")
	}
	state.serverSecret = []byte(state.cfg.authServerSecret)
	state.keyStore = newMemoryKeyStoreFromConfig(state.cfg, state.serverSecret)
	state.keyCache = auth.NewMemoryKeyCache(state.cfg.authCacheTTL, state.cfg.authNegativeCacheTTL)
	state.keyRateLimiter = auth.NewKeyRateLimiter()
	return nil
}

// memoryKeyStore is a simple in-memory KeyStore for backward compatibility
// with the single-key auth config (auth.value / COLLECTOR_API_KEY).
type memoryKeyStore struct {
	mu   sync.RWMutex
	keys map[string]*auth.KeyRecord
}

func (s *memoryKeyStore) FindByKeyID(_ context.Context, keyID string) (*auth.KeyRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec := s.keys[keyID]
	if rec == nil {
		return nil, nil
	}
	// Return a copy to prevent data races with concurrent RevokeKey mutations
	cp := *rec
	return &cp, nil
}

// CreateKey stores a new key record.
func (s *memoryKeyStore) CreateKey(rec *auth.KeyRecord) error {
	if rec == nil || rec.KeyID == "" {
		return fmt.Errorf("invalid key record")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[rec.KeyID] = rec
	return nil
}

// RevokeKey marks a key as revoked by setting RevokedAt to the current time.
func (s *memoryKeyStore) RevokeKey(keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.keys[keyID]
	if !ok {
		return fmt.Errorf("key %q not found", keyID)
	}
	now := time.Now()
	rec.RevokedAt = &now
	return nil
}
func newMemoryKeyStoreFromConfig(cfg collectorConfig, serverSecret []byte) *memoryKeyStore {
	store := &memoryKeyStore{keys: make(map[string]*auth.KeyRecord, len(cfg.authKeys)+len(cfg.authGrants)+len(cfg.authTokens)+1)}
	for _, key := range cfg.authKeys {
		var collectorGrants []auth.CollectorGrant
		if key.collector != "" {
			permissions := make(map[auth.Permission]bool, len(key.permissions))
			for _, permission := range key.permissions {
				permissions[permission] = true
			}
			collectorGrants = []auth.CollectorGrant{{
				Collector:    key.collector,
				Environments: append([]string(nil), key.allowedEnvs...),
				Permissions:  permissions,
			}}
		}
		store.keys[key.keyID] = &auth.KeyRecord{
			ID:                   key.name,
			KeyID:                key.keyID,
			SecretHash:           auth.HashSecret(key.secret, serverSecret),
			Kind:                 key.kind,
			Mode:                 key.mode,
			Roles:                append([]auth.Role(nil), key.roles...),
			CollectorGrants:      collectorGrants,
			AllowedEnvs:          append([]string(nil), key.allowedEnvs...),
			AllowedServices:      append([]string(nil), key.allowedServices...),
			AllowedOrigins:       append([]string(nil), key.allowedOrigins...),
			AllowedIPs:           append([]string(nil), key.allowedIPs...),
			MaxPayloadBytes:      key.maxPayloadBytes,
			MaxRequestsPerMinute: key.maxRequestsPerMinute,
			MaxEventsPerMinute:   key.maxEventsPerMinute,
		}
		if store.keys[key.keyID].ID == "" {
			store.keys[key.keyID].ID = key.keyID
		}
	}
	for _, grant := range cfg.authGrants {
		permissions := make(map[auth.Permission]bool, len(grant.permissions))
		for _, permission := range grant.permissions {
			permissions[permission] = true
		}
		store.keys[grant.keyID] = &auth.KeyRecord{
			ID:         grant.name,
			KeyID:      grant.keyID,
			SecretHash: auth.HashSecret(grant.secret, serverSecret),
			Kind:       grant.kind,
			CollectorGrants: []auth.CollectorGrant{{
				Collector:    grant.collector,
				Environments: append([]string(nil), grant.allowedEnvs...),
				Permissions:  permissions,
			}},
			AllowedEnvs:          append([]string(nil), grant.allowedEnvs...),
			AllowedServices:      append([]string(nil), grant.allowedServices...),
			AllowedOrigins:       append([]string(nil), grant.allowedOrigins...),
			AllowedIPs:           append([]string(nil), grant.allowedIPs...),
			MaxPayloadBytes:      grant.maxPayloadBytes,
			MaxRequestsPerMinute: grant.maxRequestsPerMinute,
			MaxEventsPerMinute:   grant.maxEventsPerMinute,
		}
		if store.keys[grant.keyID].ID == "" {
			store.keys[grant.keyID].ID = grant.keyID
		}
	}
	for _, token := range cfg.authTokens {
		tokenID := auth.TokenLookupID(token.token, serverSecret)
		permissions := make(map[auth.Permission]bool, len(token.permissions))
		for _, permission := range token.permissions {
			permissions[permission] = true
		}
		var collectorGrants []auth.CollectorGrant
		if token.collector != "" {
			collectorGrants = []auth.CollectorGrant{{
				Collector:    token.collector,
				Environments: append([]string(nil), token.allowedEnvs...),
				Permissions:  permissions,
			}}
		}
		store.keys[tokenID] = &auth.KeyRecord{
			ID:                   token.name,
			KeyID:                tokenID,
			SecretHash:           auth.HashSecret(token.token, serverSecret),
			Kind:                 auth.KeyKindToken,
			Mode:                 token.mode,
			Roles:                append([]auth.Role(nil), token.roles...),
			CollectorGrants:      collectorGrants,
			AllowedEnvs:          append([]string(nil), token.allowedEnvs...),
			AllowedServices:      append([]string(nil), token.allowedServices...),
			AllowedOrigins:       append([]string(nil), token.allowedOrigins...),
			AllowedIPs:           append([]string(nil), token.allowedIPs...),
			MaxPayloadBytes:      token.maxPayloadBytes,
			MaxRequestsPerMinute: token.maxRequestsPerMinute,
			MaxEventsPerMinute:   token.maxEventsPerMinute,
		}
		if store.keys[tokenID].ID == "" {
			store.keys[tokenID].ID = tokenID
		}
	}

	if strings.TrimSpace(cfg.apiKey) == "" {
		return store
	}
	parsed, err := auth.ParseKey(cfg.apiKey)
	if err != nil {
		return store
	}
	if _, configured := store.keys[parsed.KeyID]; configured {
		return store
	}
	store.keys[parsed.KeyID] = &auth.KeyRecord{
		ID:                   parsed.KeyID,
		KeyID:                parsed.KeyID,
		SecretHash:           auth.HashSecret(parsed.Secret, serverSecret),
		Kind:                 parsed.Kind,
		Roles:                []auth.Role{auth.RoleIngestServer},
		MaxPayloadBytes:      int(cfg.maxBodyBytes),
		MaxRequestsPerMinute: int(cfg.rateLimitRPS) * 60,
		MaxEventsPerMinute:   int(cfg.rateLimitRPS) * 600,
	}
	return store
}

func withOwnershipProjections(cfg collectorConfig) collectorConfig {
	schema := make(map[string]string, len(cfg.duckDBSchema)+2)
	for column, path := range cfg.duckDBSchema {
		schema[column] = path
	}
	schema[collectorOwnershipColumn] = collectorOwnershipColumn
	schema[environmentOwnershipColumn] = environmentOwnershipColumn
	cfg.duckDBSchema = schema

	columnTypes := make(map[string]string, len(cfg.duckDBColumnTypes)+2)
	for path, typ := range cfg.duckDBColumnTypes {
		columnTypes[path] = typ
	}
	columnTypes[collectorOwnershipColumn] = "TEXT"
	columnTypes[environmentOwnershipColumn] = "TEXT"
	cfg.duckDBColumnTypes = columnTypes

	return cfg
}

func ensureSchema(db *sql.DB, cfg collectorConfig) error {
	cfg = withOwnershipProjections(cfg)
	columns := make([]string, 0, len(cfg.duckDBSchema)+1)
	for col, path := range cfg.duckDBSchema {
		colIdent, err := quoteSQLIdent(col)
		if err != nil {
			return err
		}
		if typ, ok := cfg.duckDBColumnTypes[path]; ok {
			columns = append(columns, fmt.Sprintf("%s %s", colIdent, typ))
		} else {
			columns = append(columns, fmt.Sprintf("%s TEXT", colIdent))
		}
	}
	if cfg.duckDBStoreRaw {
		rawIdent, err := quoteSQLIdent(cfg.duckDBRawColumn)
		if err != nil {
			return err
		}
		columns = append(columns, fmt.Sprintf("%s TEXT", rawIdent))
	}
	tableIdent, err := quoteSQLIdent(cfg.duckDBTable)
	if err != nil {
		return err
	}
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableIdent, strings.Join(columns, ", "))
	if _, err := db.Exec(query); err != nil {
		return err
	}

	// Auto-migrate: add any missing blueprint columns via ALTER TABLE
	if err := autoMigrateBlueprintColumns(db, cfg); err != nil {
		return err
	}

	return nil
}

func autoMigrateBlueprintColumns(db *sql.DB, cfg collectorConfig) error {
	rows, err := db.Query("SELECT column_name FROM information_schema.columns WHERE table_name = ?", cfg.duckDBTable)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := map[string]bool{}
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return err
		}
		existing[col] = true
	}

	tableIdent, err := quoteSQLIdent(cfg.duckDBTable)
	if err != nil {
		return err
	}
	for col, path := range cfg.duckDBSchema {
		if existing[col] {
			continue
		}
		colIdent, err := quoteSQLIdent(col)
		if err != nil {
			return err
		}
		typ := "TEXT"
		if t, ok := cfg.duckDBColumnTypes[path]; ok {
			typ = t
		}
		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s", tableIdent, colIdent, typ)
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

func ensureCortexSchema(db *sql.DB) error {
	tables := []string{
		// Graph nodes
		`CREATE TABLE IF NOT EXISTS graph_nodes (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			label TEXT NOT NULL,
			attributes JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// Graph edges
		`CREATE TABLE IF NOT EXISTS graph_edges (
			id TEXT PRIMARY KEY,
			from_node_id TEXT NOT NULL,
			to_node_id TEXT NOT NULL,
			type TEXT NOT NULL,
			weight DOUBLE DEFAULT 1.0,
			attributes JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// Incidents
		`CREATE TABLE IF NOT EXISTS incidents (
			id TEXT PRIMARY KEY,
			timestamp TIMESTAMP NOT NULL,
			signature_id TEXT,
			status TEXT NOT NULL,
			severity TEXT NOT NULL,
			primary_service TEXT NOT NULL,
			affected_services JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			resolved_at TIMESTAMP
		)`,
		// Incident signatures (ALL 17 columns - fix schema drift)
		`CREATE TABLE IF NOT EXISTS incident_signatures (
			signature_id TEXT PRIMARY KEY,
			shape TEXT NOT NULL,
			service_roles JSON,
			symptoms JSON,
			temporal_pattern JSON,
			remediation JSON,
			feature_vector JSON,
			feature_weights JSON,
			occurrence_count INTEGER DEFAULT 0,
			avg_resolution_time_seconds BIGINT DEFAULT 0,
			version INTEGER DEFAULT 1,
			parent_signature_id TEXT,
			decay_factor DOUBLE DEFAULT 1.0,
			last_matched_at TIMESTAMP,
			behavioral_hash TEXT,
			embedding FLOAT[768],
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// Remediations
		`CREATE TABLE IF NOT EXISTS remediations (
			remediation_id TEXT PRIMARY KEY,
			incident_id TEXT NOT NULL,
			signature_id TEXT,
			action TEXT NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			operator TEXT,
			attributes JSON
		)`,
		// Remediation feedback (HTTP-style outcome codes)
		`CREATE TABLE IF NOT EXISTS remediation_feedback (
			feedback_id TEXT PRIMARY KEY,
			remediation_id TEXT NOT NULL,
			incident_id TEXT NOT NULL,
			outcome_code INTEGER NOT NULL,
			outcome_category TEXT NOT NULL,
			time_to_resolve_seconds BIGINT,
			timestamp TIMESTAMP NOT NULL,
			notes TEXT
		)`,
		// Topology aliases
		`CREATE TABLE IF NOT EXISTS topology_aliases (
			id TEXT PRIMARY KEY,
			alias TEXT NOT NULL,
			canonical TEXT NOT NULL,
			valid_from TIMESTAMP NOT NULL,
			valid_to TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, ddl := range tables {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("create cortex table: %w", err)
		}
	}

	// Indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_graph_nodes_type ON graph_nodes(type)",
		"CREATE INDEX IF NOT EXISTS idx_graph_edges_from ON graph_edges(from_node_id)",
		"CREATE INDEX IF NOT EXISTS idx_graph_edges_to ON graph_edges(to_node_id)",
		"CREATE INDEX IF NOT EXISTS idx_graph_edges_type ON graph_edges(type)",
		"CREATE INDEX IF NOT EXISTS idx_graph_edges_from_type ON graph_edges(from_node_id, type)",
		"CREATE INDEX IF NOT EXISTS idx_incidents_ts ON incidents(timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_incidents_svc ON incidents(primary_service)",
		"CREATE INDEX IF NOT EXISTS idx_incidents_sig ON incidents(signature_id)",
		"CREATE INDEX IF NOT EXISTS idx_signatures_hash ON incident_signatures(behavioral_hash)",
		"CREATE INDEX IF NOT EXISTS idx_topology_alias ON topology_aliases(alias)",
		"CREATE INDEX IF NOT EXISTS idx_topology_canonical ON topology_aliases(canonical)",
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("create cortex index: %w", err)
		}
	}

	return nil
}

func runPeriodicCheckpoint(db *sql.DB, interval time.Duration, stop <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if _, err := db.Exec("CHECKPOINT"); err != nil {
				logJSON("error", "collector_duckdb_periodic_checkpoint_failed", map[string]any{"error": err.Error()})
			}
		}
	}
}

func runPeriodicExport(db *sql.DB, cfg collectorConfig, stop <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	t := time.NewTicker(cfg.duckDBExportInterval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if err := exportDuckDBParquet(db, cfg); err != nil {
				logJSON("error", "collector_duckdb_export_failed", map[string]any{"error": err.Error()})
			}
		}
	}
}

func exportDuckDBParquet(db *sql.DB, cfg collectorConfig) error {
	if err := os.MkdirAll(cfg.duckDBExportPath, 0o755); err != nil {
		return err
	}
	target := storagepath.LocalParquetExportPath(cfg.duckDBExportPath, cfg.duckDBTable, time.Now().UTC())
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tableIdent, err := quoteSQLIdent(cfg.duckDBTable)
	if err != nil {
		return err
	}
	query := fmt.Sprintf("COPY %s TO %s (FORMAT PARQUET)", tableIdent, quoteSQLString(strings.ReplaceAll(target, "\\", "/")))
	_, execErr := db.Exec(query)
	return execErr
}

func quoteSQLIdent(ident string) (string, error) {
	ident = strings.TrimSpace(ident)
	if ident == "" {
		return "", fmt.Errorf("sql identifier cannot be empty")
	}
	if !configIdentPattern.MatchString(ident) {
		return "", fmt.Errorf("invalid sql identifier %q", ident)
	}
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`, nil
}

func quoteSQLString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
