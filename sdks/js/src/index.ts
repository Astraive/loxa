// --- Spec constants ---
export {
  LOXA_SPEC_VERSION,
  LOXA_EVENT_VERSION,
  LOXA_INGEST_API_VERSION,
  MAX_EVENT_BYTES,
  ALLOWED_KINDS,
  ALLOWED_LEVELS,
  ALLOWED_OUTCOMES,
  ALLOWED_EVENT_STATES,
  CANONICAL_FIELDS,
  ALLOWED_TOP_LEVEL_FIELDS,
  buildIngestEnvelope,
  normalizeEventAliases,
  parseCollectorResponse,
  isCanonical,
} from './generated/spec-contract.ts';

export type { CollectorAck, CollectorError, CollectorResponse } from './generated/spec-contract.ts';

// --- Errors ---
export {
  LoxaError,
  DuplicateEmitError,
  EventClosedError,
  EventAlreadyFinishedError,
  EventValidationError,
  extractError,
} from './core/errors.ts';

export type { ErrorInfo } from './core/errors.ts';

// --- Event types + Attr constructors ---
export {
  Event,
  // PascalCase primitive constructors
  String, Int, Int64, Uint64, Float64, Bool, Null, Any, Group, Time, Duration,
  SensitiveString, HashString, MarkSensitive,
  // PascalCase semantic shortcuts
  UserID, TenantID, WorkspaceID, OrganizationID, SessionID,
  RequestID, TraceID, SpanID,
  FeatureFlag, FeatureFlagBool, Experiment,
  OrderID, CartID, ProductID, CustomerID,
  Plan, Currency, Amount, Country, Device, Platform, AppVersion,
  ErrorType, ErrorCode, ErrorMessage, ErrorStack, Retryable,
  // PascalCase new domain helpers
  PaymentID, SubscriptionID, InvoiceID, JobID, MessageID, CorrelationID,
  CommitSha, Release,
  Money, Percent, Bytes, HttpStatus, StatusCode as StatusCodeFn, ErrorCodeExt,
  Bucket, Tags, Masked, Url, EmailHash, IpHash,
  RegionEx,
  // PascalCase domain packs
  CheckoutCartItemCount, CheckoutCartTotal, CheckoutPaymentMethod, CheckoutStatus,
  PaymentProvider, PaymentMethod, PaymentIntentId, PaymentFailureCode, PaymentRetryAttempt,
  BillingPlan, BillingSubscriptionId, BillingInvoiceId, BillingAmount, BillingInterval,
  AgentName, AgentProvider, AgentModel, AgentRunType, AgentToolName, AgentToolOutcome,
  AgentInputTokens, AgentOutputTokens, AgentCost,
  RagIndex, RagEmbeddingModel, RagChunksRetrieved, RagTopScore, RagQueryHash,
  RagCitationCount, RagRetrievalLatency,
  // camelCase aliases (primary v1 API)
  string, int, int64, uint64, float64, float, bool, null_ as nullAttr, any, group, time, duration,
  sensitiveString, hashString, markSensitive,
  userId, tenantId, workspaceId, organizationId, sessionId,
  requestId, traceId, spanId,
  featureFlag, featureFlagBool, experiment,
  orderId, cartId, productId, customerId,
  plan, currency, amount, country, device, platform, appVersion,
  errorType, errorCode, errorMessage, errorStack, retryable,
  // camelCase new domain helpers
  paymentId, subscriptionId, invoiceId, jobId, messageId, correlationId,
  commitSha, release,
  money, percent, bytes, httpStatus, statusCodeFn, errorCodeExt,
  bucket, tags, masked, url, emailHash, ipHash,
  regionEx,
  // camelCase domain packs
  checkoutCartItemCount, checkoutCartTotal, checkoutPaymentMethod, checkoutStatus,
  paymentProvider, paymentMethod, paymentIntentId, paymentFailureCode, paymentRetryAttempt,
  billingPlan, billingSubscriptionId, billingInvoiceId, billingAmount, billingInterval,
  agentName, agentProvider, agentModel, agentRunType, agentToolName, agentToolOutcome,
  agentInputTokens, agentOutputTokens, agentCost,
  ragIndex, ragEmbeddingModel, ragChunksRetrieved, ragTopScore, ragQueryHash,
  ragCitationCount, ragRetrievalLatency,
  // State constants
  EventStateCreated, EventStateActive, EventStateFinished,
  EventStateEmitting, EventStateEmitted,
  EventStateFailedValidation, EventStateDeliveryFailed,
} from './core/event.ts';

export type { Attr, AttrKind, Params, EventState } from './core/event.ts';
export {
  ProcessHandle, TimerHandle, GroupHandle, StopwatchHandle, stopwatch,
  withProcess, withGroup, withTimer,
  measure, phase, span, step,
} from './core/timing.ts';
export type { ProcessEntry, GroupEntry, TimerEntry } from './core/timing.ts';

// --- EventView ---
export { EventView } from './core/event-view.ts';

// --- Level ---
export { LevelDebug, LevelInfo, LevelNotice, LevelWarn, LevelError, LevelFatal, parseLevel, levelName } from './core/level.ts';
export type { Level } from './core/level.ts';

// --- Logger ---
export { Logger, New, TryNew, Default, Configure, getDefault, reset } from './core/logger.ts';
export type { Logger as LoggerType } from './core/logger.ts';
export { bindEvent, wrap } from './core/logger.ts';

// --- Config + Builder ---
export {
  defaultConfig, dev, development, production, test, withOptions, ConfigBuilder,
  fromEnv, disabled,
  dev as Dev, development as Development, production as Production, test as Test,
  WithService, WithAlias, WithVersion, WithEnvironment, WithSink, WithSampler,
  WithRedactor, WithSchema, WithEventSchema, WithAsync,
  WithCollectorEndpoint, WithDuplicatePolicy, WithStatsHandler,
  WithDeploymentID, WithIncludeHost, WithPanicRecovery,
  WithApiKey,
  WithRelease, WithNamespace, WithOtelBridge, WithRetry, WithTimeout, WithQueueSize, WithLogger,
} from './config/config.ts';
export type { Config, AsyncConfig, SecurityConfig, ConfigOptions } from './config/config.ts';

// --- Context ---
export { getEvent, hasEvent, eventId, requestIdFromContext, traceIdFromContext, runWithEvent, getEvent as FromContext, hasEvent as HasEvent, requestIdFromContext as RequestIDFromContext, traceIdFromContext as TraceIDFromContext } from './core/context.ts';

// --- Sink ---
export type { Sink } from './sinks/sink.ts';

// --- Standard sinks + factories ---
export {
  StdoutSink, StderrSink, FileSink, RotatingFileSink, NoopSink, MemorySink, HTTPBatchSink, CollectorSink,
  MultiSink, OtlpSink,
  stdoutSink, stderrSink, fileSink, rotatingFileSink, noopSink, memorySink, httpBatchSink, collectorSink,
  multiSink, otlpSink,
} from './sinks/standard-sinks.ts';
export type { StatsHandler, DeliveryFailureHandler, HTTPBatchSinkOptions } from './sinks/standard-sinks.ts';

// --- Redactor ---
export {
  defaultRedactor, redactKeys, dropKeys, maskKeys, composeRedactors, redactPatterns,
  hashKeys,
  defaultRedactor as DefaultRedactor,
  redactKeys as RedactKeys,
  hashKeys as HashKeys,
  dropKeys as DropKeys,
  maskKeys as MaskKeys,
  composeRedactors as ComposeRedactors,
  redactPatterns as RedactPatterns,
} from './redaction/redactor.ts';
export type { Redactor } from './redaction/redactor.ts';

// --- Sampler ---
export {
  sampleAll, sampleNone, sampleRandom, sampleErrors,
  sampleSlowRequests, sampleStatusCodes, sampleRoutes,
  sampleUsers, sampleTenants, sampleFeatureFlag,
  anySampler, allSampler, notSampler,
  sampleRateLimited, sampleByHeader,
  sampleByEvent, sampleByOutcome,
  allowFields, blockFields,
  sampleAll as SampleAll,
  sampleNone as SampleNone,
  sampleRandom as SampleRandom,
  sampleErrors as SampleErrors,
  sampleSlowRequests as SampleSlowRequests,
  sampleStatusCodes as SampleStatusCodes,
  sampleRoutes as SampleRoutes,
  sampleUsers as SampleUsers,
  sampleTenants as SampleTenants,
  sampleFeatureFlag as SampleFeatureFlag,
  sampleByHeader as SampleByHeader,
  sampleByEvent as SampleByEvent,
  sampleByOutcome as SampleByOutcome,
  allowFields as AllowFields,
  blockFields as BlockFields,
  anySampler as AnySampler,
  allSampler as AllSampler,
  notSampler as NotSampler,
} from './sampling/sampler.ts';
export type { Sampler, ShouldSample } from './sampling/sampler.ts';

// --- Schema ---
export { DefaultSchema, FlatSchema, NestedSchema, OTelLogSchema, OTelSchema, ECSchema, DatadogSchema, CustomSchema } from './core/schema.ts';
export type { Schema, SchemaFunc } from './core/schema.ts';

export { MetricsCollector, RenderPrometheus } from './metrics.ts';
export type { MetricsSnapshot } from './metrics.ts';

// --- Encoder ---
export { encodeJSON, encodePrettyJSON } from './jsonenc/encoder.ts';

// --- UUID ---
export { uuidv7 } from './core/uuidv7.ts';

// --- Collector Client ---
export { CollectorClient } from './collector/client.ts';
export type { CollectorClientOptions, VersionInfo } from './collector/client.ts';

// --- Cortex Client ---
export { CortexClient, normalizeIncidentContext, normalizeGraphView, normalizeRemediation, validateIncidentContext, validateGraphView, validateRemediation, validateFeedback } from './cortex/client.ts';
export type { CortexClientOptions, ReconstructionResult, GraphResult, Remediation, Feedback } from './cortex/client.ts';

// --- SecurityLimiter ---
export { SecurityLimiter } from './config/security.ts';
export type { SecurityConfig as SecurityLimiterConfig } from './config/security.ts';

// --- Testkit ---
export { testLogger, capture, assertEvent, assertAttr, expectEvent, expectAttr, assertRedacted, assertHasCheckpoint, snapshotEvent, MockSink, FakeClock, setIdGenerator } from './testkit/helpers.ts';
export type { TestLoggerResult } from './testkit/helpers.ts';

// --- Default facade (loxa.*) ---
export {
  loxa,
  Loxa,
  defaultLogger,
  configure,
  createLoxa,
  alias,
  startEvent, startHttpEvent, startJobEvent, startQueueEvent, startCliEvent, startCronEvent,
  append, enrich, set, merge, del, get, getGroup,
  checkpoint, process, startTimer, startGroup, finish, finishError, emit, runEvent,
  flush, shutdown,
  debug, info, warn, error, fatal,
  notice,
  event, track, audit, security, metric, count, gauge, histogram, breadcrumb,
  drop, cancel, abandon, retry, partial,
  cloneEvent, linkEvent, currentEvent,
  startEvent as StartEvent,
  startHttpEvent as StartHTTPEvent,
  startJobEvent as StartJobEvent,
  startQueueEvent as StartQueueEvent,
  startCliEvent as StartCLIEvent,
  startCronEvent as StartCronEvent,
  append as Append,
  enrich as Enrich,
  set as Set,
  merge as Merge,
  del as Delete,
  get as Get,
  getGroup as GetGroup,
  checkpoint as Checkpoint,
  process as Process,
  startTimer as StartTimer,
  startGroup as StartGroup,
  finish as Finish,
  finishError as FinishError,
  emit as Emit,
  emit as EmitEvent,
  flush as Flush,
  shutdown as Shutdown,
  debug as Debug,
  info as Info,
  notice as Notice,
  warn as Warn,
  error as Error,
  fatal as Fatal,
  event as EventLogger,
  track as Track,
  audit as Audit,
  security as Security,
  metric as Metric,
  count as Count,
  gauge as Gauge,
  histogram as Histogram,
  breadcrumb as Breadcrumb,
  drop as Drop,
  cancel as Cancel,
  abandon as Abandon,
  retry as Retry,
  partial as Partial,
  cloneEvent as CloneEvent,
  linkEvent as LinkEvent,
  currentEvent as CurrentEvent,
  createLoxa as CreateLoxa,
  alias as Alias,
} from './loxa.ts';
