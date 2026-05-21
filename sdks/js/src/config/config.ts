import type { Sink } from '../sinks/sink.ts';
import type { Redactor } from '../redaction/redactor.ts';
import { defaultRedactor } from '../redaction/redactor.ts';
import type { Sampler } from '../sampling/sampler.ts';
import { sampleAll } from '../sampling/sampler.ts';
import type { Schema } from '../core/schema.ts';
import { DefaultSchema } from '../core/schema.ts';

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
  version: string;
  environment: string;
  collectorUrl: string;
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
}

/** Default configuration. */
export function defaultConfig(): Config {
  return {
    service: '',
    version: '',
    environment: 'development',
    collectorUrl: '',
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
  };
}

/** Development preset. */
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
  version?: string;
  environment?: string;
  collectorUrl?: string;
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
  version: string;
  environment: string;
  collectorUrl: string;
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

  constructor(base: Config) {
    this.service = base.service;
    this.version = base.version;
    this.environment = base.environment;
    this.collectorUrl = base.collectorUrl;
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
  }

  withService(service: string): this { this.service = service; return this; }
  withVersion(version: string): this { this.version = version; return this; }
  withEnvironment(environment: string): this { this.environment = environment; return this; }
  withCollectorUrl(url: string): this { this.collectorUrl = url; return this; }
  withSink(sink: Sink): this { this.sink = sink; return this; }
  withSinks(...sinks: Sink[]): this { this.sinks = sinks; return this; }
  withSampler(sampler: Sampler): this { this.sampler = sampler; return this; }
  withRedactor(redactor: Redactor): this { this.redactor = redactor; return this; }
  withSchema(schema: Schema): this { this.schema = schema; return this; }
  withLevel(level: string): this { this.level = level; return this; }
  withStrict(strict: boolean): this { this.strict = strict; return this; }
  withAsync(enabled: boolean): this { this.async.enabled = enabled; return this; }
  withCollectorEndpoint(url: string): this { this.collectorUrl = url; return this; }
  withDuplicatePolicy(policy: string): this { this.duplicatePolicy = policy; return this; }
  withStatsHandler(_handler: unknown): this { return this; }
  withDeploymentID(_deploymentId: string): this { return this; }
  withIncludeHost(includeHost: boolean): this { this.includeHost = includeHost; return this; }
  withPanicRecovery(_panicRecovery: boolean): this { return this; }
  withBatchSize(n: number): this { this.batchSize = n; return this; }
  withFlushInterval(ms: number): this { this.flushIntervalMs = ms; return this; }
  withEnableCompression(enabled: boolean): this { this.enableCompression = enabled; return this; }
  build(): Config { return { ...this }; }
}

/** Create a config from a base config and options. */
export function withOptions(base: Config, opts: ConfigOptions): Config {
  const cfg = { ...base };
  if (opts.service !== undefined) cfg.service = opts.service;
  if (opts.version !== undefined) cfg.version = opts.version;
  if (opts.environment !== undefined) cfg.environment = opts.environment;
  if (opts.collectorUrl !== undefined) cfg.collectorUrl = opts.collectorUrl;
  if (opts.sink !== undefined) cfg.sink = opts.sink;
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
  return cfg;
}

export type ConfigOption = (cfg: Config) => Config;

export function WithService(service: string): ConfigOption { return cfg => withOptions(cfg, { service }); }
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
export function WithStatsHandler(statsHandler: unknown): ConfigOption { return cfg => withOptions(cfg, { statsHandler }); }
export function WithDeploymentID(deploymentId: string): ConfigOption { return cfg => withOptions(cfg, { deploymentId }); }
export function WithIncludeHost(includeHost: boolean): ConfigOption { return cfg => withOptions(cfg, { includeHost }); }
export function WithPanicRecovery(panicRecovery: boolean): ConfigOption { return cfg => withOptions(cfg, { panicRecovery }); }
