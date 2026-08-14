import type { Logger } from '../core/logger.ts';
import type { Sink } from '../sinks/sink.ts';
import type { Redactor } from '../redaction/redactor.ts';
import { defaultRedactor } from '../redaction/redactor.ts';
import type { Sampler } from '../sampling/sampler.ts';
import { sampleAll, sampleNone } from '../sampling/sampler.ts';
import type { Schema } from '../core/schema.ts';
import { DefaultSchema } from '../core/schema.ts';
import type { Level } from '../core/level.ts';
import { loadFileConfig, mergeFileConfig } from './config-file.ts';
import { isPublicDSNUsername, parse as parseDSN } from './dsn.ts';

function isLocalhost(host: string): boolean {
  return host === 'localhost' || host === '127.0.0.1' || host === '::1';
}

function hasBasicCredentials(cfg: Pick<Config, 'apiKey' | 'username' | 'password'>): boolean {
  return !cfg.apiKey && !!cfg.username && (!!cfg.password || isPublicDSNUsername(cfg.username));
}

export function collectorRouteURL(baseURL: string, collectorName: string, route: string): string {
  const base = baseURL.replace(/\/+$/, '');
  return collectorName ? `${base}/collectors/${encodeURIComponent(collectorName)}${route}` : `${base}${route}`;
}

/** Apply a collector endpoint, resolving loza:// DSNs without retaining userinfo. */
function applyCollectorURL(cfg: Config, raw: string): void {
  if (!raw.startsWith('loza://')) {
    cfg.collectorUrl = raw;
    return;
  }
  const dsn = parseDSN(raw);
  cfg.collectorUrl = dsn.baseURL;
  cfg.collectorName = dsn.collectorName;
  if (dsn.username !== undefined) {
    cfg.username = dsn.username;
    cfg.password = dsn.password ?? '';
  }
  if (dsn.env && dsn.env !== 'default') cfg.environment = dsn.env;
  if (dsn.service) cfg.service = dsn.service;
}

/** Reject credentialed non-local plaintext HTTP before any request is made. */
export function validateConfig(cfg: Config): Config {
  if (!cfg.collectorUrl) return cfg;
  const endpoint = new URL(cfg.collectorUrl);
  if (endpoint.username || endpoint.password) {
    throw new Error('invalid Loza config: credentials must not be embedded in the collector URL');
  }
  if (!cfg.username && cfg.password) {
    throw new Error('invalid Loza config: Basic password requires a username');
  }
  if (cfg.username && !cfg.password && !isPublicDSNUsername(cfg.username)) {
    throw new Error('invalid Loza config: Basic credentials require a password unless username is an lx_pub_ capability');
  }
  if (hasBasicCredentials(cfg) && endpoint.protocol === 'http:' && !isLocalhost(endpoint.hostname)) {
    throw new Error('invalid Loza config: Basic credentials require HTTPS (HTTP is allowed only for localhost)');
  }
  return cfg;
}

/** Async delivery configuration. */
export interface AsyncConfig {
  enabled: boolean;
  queueSize: number;
  workers: number;
  maxBatchBytes: number;
  flushIntervalMs: number;
}

/** Security limits. */
export interface SecurityConfig {
  redactByDefault: boolean;
  allowPII: boolean;
  maxFieldBytes: number;
  maxEventBytes: number;
  maxAttrCount: number;
  dropOversizedEvents: boolean;
}

/** Top-level SDK configuration. */
export interface Config {
  service: string;
  alias: string;
  version: string;
  release: string;
  environment: string;
  namespace: string;
  collectorUrl: string;
  collectorName: string;
  apiKey: string;
  username: string;
  password: string;
  sink: Sink | null;
  sinks: Sink[];
  sampler: Sampler;
  redactor: Redactor | null;
  schema: Schema;
  level: string;
  strict: boolean;
  async: AsyncConfig;
  security: SecurityConfig;
  batchSize: number;
  flushIntervalMs: number;
  maxRetries: number;
  maxBackoffMs: number;
  timeoutMs: number;
  enableCompression: boolean;
  includeHost: boolean;
  includeRuntime: boolean;
  duplicatePolicy: string;
  logger: Logger | null;
}

/** Default configuration. */
/** Create a disabled config (no-op). */
export function disabled(): Config {
  return { ...defaultConfig(), sink: null, sampler: sampleNone() };
}

/**
 * Load config using the 4-layer precedence:
 *   1. Hardcoded defaults (defaultConfig)
 *   2. YAML files (loza-js.defaults.yaml + user override)
 *   3. Environment variables (including LOZA_DSN)
 *   4. Code-level config (via builder / withOptions)
 *
 * This function implements layers 1-3. Layer 4 is applied by the caller.
 */
export function fromEnv(): Config {
  const cfg = defaultConfig();

  // Layer 2: YAML file config (defaults + user overlay)
  try {
    const fileRaw = loadFileConfig();
    if (Object.keys(fileRaw).length > 0) {
      const merged = mergeFileConfig(cfg, fileRaw);
      Object.assign(cfg, merged);
    }
  } catch {
    // File loading failed silently — continue with hardcoded defaults
  }

  // Layer 3: Environment variables
  if (typeof process !== 'undefined') {
    // Parse LOZA_DSN first. Resolved URLs never retain userinfo.
    const dsnRaw = process.env.LOZA_DSN;
    if (dsnRaw) {
      try {
        applyCollectorURL(cfg, dsnRaw);
      } catch {
        // Invalid DSN — fall through to individual env vars
      }
    }

    // Individual env vars override DSN-derived and file-derived values.
    // LOZA_COLLECTOR_URL intentionally overrides only the endpoint; DSN
    // environment/service/credentials remain in effect.
    cfg.service = process.env.LOZA_SERVICE || process.env.SERVICE || cfg.service;
    cfg.version = process.env.LOZA_VERSION || process.env.VERSION || cfg.version;
    cfg.environment = process.env.LOZA_ENVIRONMENT || process.env.ENVIRONMENT || cfg.environment;
    cfg.release = process.env.LOZA_RELEASE || process.env.RELEASE || cfg.release;
    cfg.namespace = process.env.LOZA_NAMESPACE || process.env.NAMESPACE || cfg.namespace;
    cfg.collectorUrl = process.env.LOZA_COLLECTOR_URL || process.env.COLLECTOR_URL || cfg.collectorUrl;
    cfg.apiKey = process.env.LOZA_API_KEY || process.env.API_KEY || cfg.apiKey;
    cfg.level = process.env.LOZA_LEVEL || process.env.LOG_LEVEL || cfg.level;
  }
  return validateConfig(cfg);
}

export function defaultConfig(): Config {
  return {
    service: '',
    alias: '',
    version: '',
    environment: 'development',
    release: '',
    namespace: '',
    collectorUrl: '',
    collectorName: '',
    apiKey: (typeof process !== 'undefined' && process.env?.LOZA_API_KEY) || '',
    username: '',
    password: '',
    sink: null,
    sinks: [],
    sampler: sampleAll(),
    redactor: null,
    schema: new DefaultSchema(),
    level: 'info',
    strict: false,
    async: {
      enabled: false,
      queueSize: 8192,
      workers: 1,
      maxBatchBytes: 256 * 1024,
      flushIntervalMs: 100,
    },
    security: {
      redactByDefault: true,
      allowPII: false,
      maxFieldBytes: 4096,
      maxEventBytes: 256 * 1024,
      maxAttrCount: 512,
      dropOversizedEvents: true,
    },
    batchSize: 50,
    flushIntervalMs: 5000,
    maxRetries: 3,
    maxBackoffMs: 30000,
    timeoutMs: 5000,
    enableCompression: true,
    includeHost: true,
    includeRuntime: true,
    duplicatePolicy: 'canonical_wins',
    logger: null,
  };
}

/** Development preset. */
export function development(service: string): ConfigBuilder {
  return new ConfigBuilder({ ...defaultConfig(), service, environment: 'development' });
}

/** Alias for development(). */
export function dev(service: string): ConfigBuilder {
  return new ConfigBuilder({ ...defaultConfig(), service, environment: 'development' });
}

/** Production preset. */
export function production(service: string): ConfigBuilder {
  return new ConfigBuilder({
    ...defaultConfig(),
    service,
    environment: 'production',
    async: { ...defaultConfig().async, enabled: true },
    strict: true,
  });
}

/** Test preset. */
export function test(service: string = 'test'): ConfigBuilder {
  return new ConfigBuilder({ ...defaultConfig(), service, environment: 'test' });
}

/** Config builder options. */
export interface ConfigOptions {
  service?: string;
  alias?: string;
  version?: string;
  environment?: string;
  release?: string;
  namespace?: string;
  collectorUrl?: string;
  collectorName?: string;
  apiKey?: string;
  username?: string;
  password?: string;
  sink?: Sink;
  sampler?: Sampler;
  redactor?: Redactor;
  schema?: Schema;
  level?: string;
  strict?: boolean;
  async?: boolean;
  asyncQueueSize?: number;
  workers?: number;
  batchSize?: number;
  flushIntervalMs?: number;
  enableCompression?: boolean;
  duplicatePolicy?: string;
  statsHandler?: unknown;
  deploymentId?: string;
  includeHost?: boolean;
  panicRecovery?: boolean;
}

/** Fluent config builder. Implements Config so it can be passed directly to Logger/configure(). */
export class ConfigBuilder implements Config {
  service: string;
  alias: string;
  version: string;
  release: string;
  environment: string;
  namespace: string;
  collectorUrl: string;
  collectorName: string;
  apiKey: string;
  username: string;
  password: string;
  sink: Sink | null;
  sinks: Sink[];
  sampler: Sampler;
  redactor: Redactor | null;
  schema: Schema;
  level: string;
  strict: boolean;
  async: AsyncConfig;
  security: SecurityConfig;
  batchSize: number;
  flushIntervalMs: number;
  maxRetries: number;
  maxBackoffMs: number;
  timeoutMs: number;
  enableCompression: boolean;
  includeHost: boolean;
  includeRuntime: boolean;
  duplicatePolicy: string;
  logger: Logger | null;

  constructor(base: Config) {
    this.service = base.service;
    this.alias = base.alias;
    this.version = base.version;
    this.release = base.release;
    this.environment = base.environment;
    this.namespace = base.namespace;
    this.collectorUrl = base.collectorUrl;
    this.collectorName = base.collectorName;
    this.apiKey = base.apiKey;
    this.username = base.username;
    this.password = base.password;
    this.sink = base.sink;
    this.sinks = base.sinks;
    this.sampler = base.sampler;
    this.redactor = base.redactor;
    this.schema = base.schema;
    this.level = base.level;
    this.strict = base.strict;
    this.async = { ...base.async };
    this.security = { ...base.security };
    this.batchSize = base.batchSize;
    this.flushIntervalMs = base.flushIntervalMs;
    this.maxRetries = base.maxRetries;
    this.maxBackoffMs = base.maxBackoffMs;
    this.timeoutMs = base.timeoutMs;
    this.enableCompression = base.enableCompression;
    this.includeHost = base.includeHost;
    this.includeRuntime = base.includeRuntime;
    this.duplicatePolicy = base.duplicatePolicy;
    this.logger = base.logger;
  }

  withService(service: string): this { this.service = service; return this; }
  withAlias(alias: string): this { this.alias = alias; return this; }
  withVersion(version: string): this { this.version = version; return this; }
  withEnvironment(environment: string): this { this.environment = environment; return this; }
  withCollectorUrl(url: string): this {
    Object.assign(this, withOptions(this, { collectorUrl: url }));
    return this;
  }
  withCollectorName(collectorName: string): this { this.collectorName = collectorName; return this; }
  withApiKey(apiKey: string): this { this.apiKey = apiKey.trim(); return this; }
  withBasicAuth(username: string, password: string): this {
    this.username = username;
    this.password = password;
    return this;
  }
  withSink(sink: Sink): this { this.sink = sink; return this; }
  withSinks(...sinks: Sink[]): this { this.sinks = sinks; return this; }
  withSampler(sampler: Sampler): this { this.sampler = sampler; return this; }
  withRedactor(redactor: Redactor): this { this.redactor = redactor; return this; }
  withSchema(schema: Schema): this { this.schema = schema; return this; }
  withLevel(level: string): this { this.level = level; return this; }
  withStrict(strict: boolean): this { this.strict = strict; return this; }
  withDuplicatePolicy(policy: string): this { this.duplicatePolicy = policy; return this; }
  withAsync(enabled: boolean): this { this.async.enabled = enabled; return this; }
  withCollectorEndpoint(url: string): this { return this.withCollectorUrl(url); }
  withStatsHandler(_handler: unknown): this { return this; }
  withDeploymentID(_deploymentId: string): this { return this; }
  withIncludeHost(includeHost: boolean): this { this.includeHost = includeHost; return this; }
  withPanicRecovery(_panicRecovery: boolean): this { return this; }
  withBatchSize(n: number): this { this.batchSize = n; return this; }
  withFlushInterval(ms: number): this { this.flushIntervalMs = ms; return this; }
  withEnableCompression(enabled: boolean): this { this.enableCompression = enabled; return this; }
  withRelease(release: string): this { this.release = release; return this; }
  withNamespace(ns: string): this { this.namespace = ns; return this; }
  withOtelBridge(enabled: boolean): this {
    if (enabled) this.async.enabled = true;
    return this;
  }
  withRetry(retries: number): this { this.maxRetries = retries; return this; }
  withTimeout(ms: number): this { this.timeoutMs = ms; return this; }
  withQueueSize(size: number): this { this.async.queueSize = size; return this; }
  withLogger(logger: Logger): this { this.logger = logger; return this; }
  disabled(): Config { return { ...this, sink: null, sampler: sampleNone() }; }
  build(): Config { return validateConfig({ ...this }); }
}

/** Create a config from a base config and options. */
export function withOptions(base: Config, opts: ConfigOptions): Config {
  const cfg = { ...base };
  if (opts.service !== undefined) cfg.service = opts.service;
  if (opts.alias !== undefined) cfg.alias = opts.alias;
  if (opts.version !== undefined) cfg.version = opts.version;
  if (opts.environment !== undefined) cfg.environment = opts.environment;
  if (opts.release !== undefined) cfg.release = opts.release;
  if (opts.namespace !== undefined) cfg.namespace = opts.namespace;
  if (opts.collectorUrl !== undefined) applyCollectorURL(cfg, opts.collectorUrl);
  if (opts.collectorName !== undefined) cfg.collectorName = opts.collectorName;
  if (opts.apiKey !== undefined) cfg.apiKey = opts.apiKey.trim();
  if (opts.username !== undefined) cfg.username = opts.username;
  if (opts.password !== undefined) cfg.password = opts.password;
  if (opts.sampler !== undefined) cfg.sampler = opts.sampler;
  if (opts.redactor !== undefined) cfg.redactor = opts.redactor;
  if (opts.schema !== undefined) cfg.schema = opts.schema;
  if (opts.level !== undefined) cfg.level = opts.level;
  if (opts.strict !== undefined) cfg.strict = opts.strict;
  if (opts.async !== undefined) cfg.async = { ...cfg.async, enabled: opts.async };
  if (opts.asyncQueueSize !== undefined) cfg.async = { ...cfg.async, enabled: true, queueSize: opts.asyncQueueSize };
  if (opts.workers !== undefined) cfg.async = { ...cfg.async, workers: opts.workers };
  if (opts.batchSize !== undefined) cfg.batchSize = opts.batchSize;
  if (opts.flushIntervalMs !== undefined) cfg.flushIntervalMs = opts.flushIntervalMs;
  if (opts.enableCompression !== undefined) cfg.enableCompression = opts.enableCompression;
  if (opts.duplicatePolicy !== undefined) cfg.duplicatePolicy = opts.duplicatePolicy;
  if (opts.includeHost !== undefined) cfg.includeHost = opts.includeHost;
  return validateConfig(cfg);
}

export type ConfigOption = (cfg: Config) => Config;

export function WithService(service: string): ConfigOption { return cfg => withOptions(cfg, { service }); }
export function WithAlias(alias: string): ConfigOption { return cfg => withOptions(cfg, { alias }); }
export function WithVersion(version: string): ConfigOption { return cfg => withOptions(cfg, { version }); }
export function WithEnvironment(environment: string): ConfigOption { return cfg => withOptions(cfg, { environment }); }
export function WithSink(sink: Sink): ConfigOption { return cfg => withOptions(cfg, { sink }); }
export function WithSampler(sampler: Sampler): ConfigOption { return cfg => withOptions(cfg, { sampler }); }
export function WithRedactor(redactor: Redactor): ConfigOption { return cfg => withOptions(cfg, { redactor }); }
export function WithSchema(schema: Schema): ConfigOption { return cfg => withOptions(cfg, { schema }); }
export function WithEventSchema(schema: Schema): ConfigOption { return WithSchema(schema); }
export function WithAsync(enabled: boolean): ConfigOption { return cfg => withOptions(cfg, { async: enabled }); }
export function WithCollectorEndpoint(collectorUrl: string): ConfigOption { return cfg => withOptions(cfg, { collectorUrl }); }
export function WithDuplicatePolicy(duplicatePolicy: string): ConfigOption { return cfg => withOptions(cfg, { duplicatePolicy }); }

export function WithBasicAuth(username: string, password: string): ConfigOption {
  return cfg => withOptions(cfg, { username, password });
}
export function WithStatsHandler(statsHandler: unknown): ConfigOption { return cfg => withOptions(cfg, { statsHandler }); }
export function WithDeploymentID(deploymentId: string): ConfigOption { return cfg => withOptions(cfg, { deploymentId }); }
export function WithIncludeHost(includeHost: boolean): ConfigOption { return cfg => withOptions(cfg, { includeHost }); }
export function WithPanicRecovery(panicRecovery: boolean): ConfigOption { return cfg => withOptions(cfg, { panicRecovery }); }
export function WithApiKey(apiKey: string): ConfigOption { return cfg => { cfg.apiKey = apiKey.trim(); return cfg; }; }
export function WithRelease(_release: string): ConfigOption { return cfg => cfg; }
export function WithNamespace(_ns: string): ConfigOption { return cfg => cfg; }
export function WithOtelBridge(enabled: boolean): ConfigOption {
  return cfg => enabled ? { ...cfg, async: { ...cfg.async, enabled: true } } : cfg;
}
export function WithRetry(retries: number): ConfigOption { return cfg => { cfg.maxRetries = retries; return cfg; }; }
export function WithTimeout(ms: number): ConfigOption { return cfg => { cfg.timeoutMs = ms; return cfg; }; }
export function WithQueueSize(size: number): ConfigOption { return cfg => { cfg.async.queueSize = size; return cfg; }; }
export function WithLogger(_logger: Logger): ConfigOption { return cfg => cfg; }
