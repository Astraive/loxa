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
  // camelCase aliases (primary v1 API)
  string, int, int64, uint64, float64, bool, any, group, time, duration,
  sensitiveString, hashString, markSensitive,
  userId, tenantId, workspaceId, organizationId, sessionId,
  requestId, traceId, spanId,
  featureFlag, featureFlagBool, experiment,
  orderId, cartId, productId, customerId,
  plan, currency, amount, country, device, platform, appVersion,
  errorType, errorCode, errorMessage, errorStack, retryable,
  // State constants
  EventStateCreated, EventStateActive, EventStateFinished,
  EventStateEmitting, EventStateEmitted,
  EventStateFailedValidation, EventStateDeliveryFailed,
} from './core/event.ts';

export type { Attr, AttrKind, Params, EventState } from './core/event.ts';
export { ProcessHandle, TimerHandle, GroupHandle, StopwatchHandle, stopwatch } from './core/timing.ts';
export type { ProcessEntry, GroupEntry, TimerEntry } from './core/timing.ts';

// --- EventView ---
export { EventView } from './core/event-view.ts';

// --- Level ---
export { LevelDebug, LevelInfo, LevelWarn, LevelError, LevelFatal, parseLevel, levelName } from './core/level.ts';
export type { Level } from './core/level.ts';

// --- Logger ---
export { Logger, New, TryNew, Default, Configure, getDefault, configure, reset } from './core/logger.ts';

// --- Config + Builder ---
export {
  defaultConfig, dev, production, test, withOptions, ConfigBuilder,
  dev as Dev, production as Production, test as Test,
  WithService, WithVersion, WithEnvironment, WithSink, WithSampler,
  WithRedactor, WithSchema, WithEventSchema, WithAsync,
  WithCollectorEndpoint, WithDuplicatePolicy, WithStatsHandler,
  WithDeploymentID, WithIncludeHost, WithPanicRecovery,
} from './config/config.ts';
export type { Config, AsyncConfig, SecurityConfig, ConfigOptions } from './config/config.ts';

// --- Context ---
export { getEvent, hasEvent, eventId, runWithEvent, getEvent as FromContext, hasEvent as HasEvent } from './core/context.ts';

// --- Sink ---
export type { Sink } from './sinks/sink.ts';

// --- Standard sinks + factories ---
export {
  StdoutSink, StderrSink, FileSink, RotatingFileSink, NoopSink, MemorySink, HTTPBatchSink, CollectorSink,
  stdoutSink, stderrSink, fileSink, rotatingFileSink, noopSink, memorySink, httpBatchSink, collectorSink,
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
  anySampler as AnySampler,
  allSampler as AllSampler,
  notSampler as NotSampler,
} from './sampling/sampler.ts';
export type { Sampler } from './sampling/sampler.ts';

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

// --- Default facade (loxa.*) ---
export {
  defaultLogger,
  startEvent, startHttpEvent, startJobEvent, startQueueEvent, startCliEvent, startCronEvent,
  append, enrich, set, merge, del, get, getGroup,
  checkpoint, finish, finishError, emit, runEvent,
  flush, shutdown,
  debug, info, warn, error, fatal,
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
  finish as Finish,
  finishError as FinishError,
  emit as Emit,
  emit as EmitEvent,
  flush as Flush,
  shutdown as Shutdown,
  debug as Debug,
  info as Info,
  warn as Warn,
  error as Error,
  fatal as Fatal,
} from './loxa.ts';
