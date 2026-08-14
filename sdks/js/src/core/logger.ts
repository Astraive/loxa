import {
  Event, EventStateEmitting, EventStateEmitted,
  Float64, Int64,
  String as StringAttr,
} from './event.ts';
import type { Attr, Params } from './event.ts';
import { collectorRouteURL, defaultConfig, withOptions, dev, production, test, validateConfig } from '../config/config.ts';
import type { Config, ConfigOptions } from '../config/config.ts';
import type { Sink } from '../sinks/sink.ts';
import { HTTPBatchSink } from '../sinks/standard-sinks.ts';
import { encodeJSON } from '../jsonenc/encoder.ts';
import { storeEvent, getEvent, runWithEvent } from './context.ts';
import { LevelDebug, LevelInfo, LevelNotice, LevelWarn, LevelError, LevelFatal, parseLevel } from './level.ts';
import type { Level } from './level.ts';
import { sanitizeEvent } from './sanitize.ts';
import { EventView } from './event-view.ts';
import type { ProcessHandle, GroupHandle, TimerHandle } from './timing.ts';

/** Resolve the effective sink: explicit sink > HTTPBatchSink from collectorUrl > null. */
function resolveSink(cfg: Config): Sink | null {
  if (cfg.sink) return cfg.sink;
  if (cfg.collectorUrl) {
    return new HTTPBatchSink({
      endpoint: collectorRouteURL(cfg.collectorUrl, cfg.collectorName, '/events'),
      apiKey: cfg.apiKey,
      username: cfg.username,
      password: cfg.password,
      service: cfg.service,
      batchSize: cfg.batchSize,
      flushIntervalMs: cfg.flushIntervalMs,
      retries: cfg.maxRetries,
      baseDelay: 100,
      maxDelay: cfg.maxBackoffMs,
      timeout: cfg.timeoutMs,
      enableCompression: cfg.enableCompression,
    });
  }
  return null;
}

/** The core LOZA logger. */
export class Logger {
  private cfg: Config;
  private _resolvedSink: Sink | null;

  constructor(cfg?: Partial<Config>) {
    this.cfg = validateConfig({ ...defaultConfig(), ...cfg });
    this._resolvedSink = resolveSink(this.cfg);
  }

  /** Create a child logger with modified config. */
  child(opts: ConfigOptions): Logger {
    return new Logger(withOptions(this.cfg, opts));
  }

  /** Create an immutable child logger that preserves config and emits loza.alias. */
  alias(name: string): Logger {
    return this.child({ alias: name });
  }

  /** Get the current config. */
  getConfig(): Config { return { ...this.cfg }; }

  // --- Event lifecycle ---

  /** Start a new event and store it in async context. */
  startEvent(params: Params): Event {
    const ev = new Event(params, this.cfg.service, this.cfg.environment);
    if (!ev.service && this.cfg.service) ev.service = this.cfg.service;
    if (!ev.version && this.cfg.version) ev.version = this.cfg.version;
    if (!ev.environment && this.cfg.environment) ev.environment = this.cfg.environment;
    if (this.cfg.alias) ev.set('loza.alias', this.cfg.alias);

    // Apply custom attrs
    if (params.custom) {
      for (const attr of params.custom) {
        ev.attrs[attr.key] = attr.value;
      }
    }

    storeEvent(ev);
    return ev;
  }

  /** Enrich the current event with attributes. */
  enrich(ctx: Event, ...attrs: Attr[]): void {
    ctx.enrich(...attrs);
  }

  /** Alias for enrich — the v1-preferred name. */
  append(ctx: Event, ...attrs: Attr[]): void {
    ctx.enrich(...attrs);
  }

  /** Set a single attr by key on the event. */
  set(ctx: Event, key: string, value: any): void {
    ctx.set(key, value);
  }

  /** Merge a plain object's entries into the event's attrs. */
  merge(ctx: Event, obj: Record<string, any>): void {
    ctx.merge(obj);
  }

  /** Remove an attr by key from the event. */
  delete(ctx: Event, key: string): void {
    ctx.delete(key);
  }

  /** Return the value for a key from the event's attrs. */
  get(ctx: Event, key: string): any {
    return ctx.get(key);
  }

  /** Return all attrs whose key starts with prefix., with prefix stripped. */
  getGroup(ctx: Event, prefix: string): Record<string, any> {
    return ctx.getGroup(prefix);
  }

  /** Record a checkpoint on the current event. */
  checkpoint(ctx: Event, name: string, attrs?: Record<string, any>): void {
    ctx.checkpoint(name, attrs);
  }

  /** Start a named process step and return a handle. */
  process(ctx: Event, name: string, ...attrs: Attr[]): ProcessHandle {
    return ctx.startProcess(name, ...attrs);
  }
  startProcess(ctx: Event, name: string, ...attrs: Attr[]): ProcessHandle {
    return this.process(ctx, name, ...attrs);
  }
  finishProcess(handle: ProcessHandle, ...attrs: Attr[]): void {
    handle.finish(...attrs);
  }
  finishProcessError(handle: ProcessHandle, err: unknown, ...attrs: Attr[]): void {
    handle.finishError(err, ...attrs);
  }

  /** Start a named group and return a handle. */
  group(ctx: Event, name: string, ...attrs: Attr[]): GroupHandle {
    return ctx.startGroup(name, ...attrs);
  }
  startGroup(ctx: Event, name: string, ...attrs: Attr[]): GroupHandle {
    return this.group(ctx, name, ...attrs);
  }
  finishGroup(handle: GroupHandle, ...attrs: Attr[]): void {
    handle.finish(...attrs);
  }
  finishGroupError(handle: GroupHandle, err: unknown, ...attrs: Attr[]): void {
    handle.finishError(err, ...attrs);
  }

  /** Start a named timer and return a handle. */
  timer(ctx: Event, name: string, ...attrs: Attr[]): TimerHandle {
    return ctx.startTimer(name, ...attrs);
  }
  startTimer(ctx: Event, name: string, ...attrs: Attr[]): TimerHandle {
    return this.timer(ctx, name, ...attrs);
  }
  stopTimer(handle: TimerHandle, ...attrs: Attr[]): void {
    handle.stop(...attrs);
  }

  /** Finish the event with an outcome. */
  finish(ctx: Event, outcome: string, ...attrs: Attr[]): void {
    ctx.finish(outcome, ...attrs);
  }

  /** Finish the event with an error. */
  finishError(ctx: Event, err: unknown, ...attrs: Attr[]): void {
    ctx.finishError(err, ...attrs);
  }

  /** Emit the event — sanitize, encode, apply redactor, deliver to sink. */
  async emit(ctx: Event): Promise<string | null> {
    if (ctx.emitted) return null; // idempotent

    if (!ctx.markEmitted()) return null;

    // Apply sampler
    if (this.cfg.sampler && !this.cfg.sampler(ctx)) {
      ctx.markEmittedDone();
      return null;
    }

    // Sanitize: clone + apply sensitive/hash/drop
    const sanitized = sanitizeEvent(ctx);

    // Create immutable view for schema encoding
    const view = new EventView(sanitized);

    // Encode via schema
    let payload = this.cfg.schema.encode(view);

    // Apply redactor
    if (this.cfg.redactor) {
      payload = this.cfg.redactor(payload);
    }

    // Serialize
    const encoded = encodeJSON(payload);

    // Deliver to sink
    const sink = this._resolvedSink;
    if (sink) {
      try {
        await sink.write(encoded);
      } catch (err) {
        ctx.markDeliveryFailed();
        console.error('[loza] sink write failed:', err);
        return null;
      }
    }

    ctx.markEmittedDone();
    return encoded;
  }

  /** Start, run fn, finish, and emit in one call. */
  async runEvent(params: Params, fn: (ctx: Event) => void | Promise<void>, finishAttrs: Attr[] = []): Promise<string | null> {
    const ctx = this.startEvent(params);
    try {
      await fn(ctx);
      if (!ctx.getEventState().startsWith('finished')) {
        ctx.finish('success', ...finishAttrs);
      }
    } catch (err) {
      ctx.finishError(err, ...finishAttrs);
    }
    return this.emit(ctx);
  }

  /** Run fn with existing event, auto finish/emit. */
  async run(ctx: Event, fn: (ctx: Event) => void | Promise<void>, finishAttrs: Attr[] = []): Promise<string | null> {
    try {
      await fn(ctx);
      if (!ctx.getEventState().startsWith('finished')) {
        ctx.finish('success', ...finishAttrs);
      }
    } catch (err) {
      ctx.finishError(err, ...finishAttrs);
    }
    return this.emit(ctx);
  }

  /** Start an HTTP event. */
  startHTTPEvent(params: Params): Event {
    return this.startEvent({ ...params, kind: 'http' });
  }

  /** Start a job event. */
  startJobEvent(params: Params): Event {
    return this.startEvent({ ...params, kind: 'job' });
  }

  /** Start a queue event. */
  startQueueEvent(params: Params): Event {
    return this.startEvent({ ...params, kind: 'queue' });
  }

  /** Start a CLI event. */
  startCLIEvent(params: Params): Event {
    return this.startEvent({ ...params, kind: 'cli' });
  }

  /** Start a cron event. */
  startCronEvent(params: Params): Event {
    return this.startEvent({ ...params, kind: 'cron' });
  }

  // --- Immediate log helpers ---

  private async immediate(level: Level, message: string, attrs: Attr[] = []): Promise<void> {
    const ctx = this.startEvent({ event: message, level, message });
    if (attrs.length > 0) ctx.enrich(...attrs);
    ctx.finish('success');
    await this.emit(ctx);
  }

  debug(message: string, ...attrs: Attr[]): Promise<void> { return this.immediate(LevelDebug, message, attrs); }
  info(message: string, ...attrs: Attr[]): Promise<void> { return this.immediate(LevelInfo, message, attrs); }
  notice(message: string, ...attrs: Attr[]): Promise<void> { return this.immediate(LevelNotice, message, attrs); }
  warn(message: string, ...attrs: Attr[]): Promise<void> { return this.immediate(LevelWarn, message, attrs); }
  error(message: string, ...attrs: Attr[]): Promise<void> { return this.immediate(LevelError, message, attrs); }
  fatal(message: string, ...attrs: Attr[]): Promise<void> { return this.immediate(LevelFatal, message, attrs); }

  // Logging helpers
  async event(name: string, ...attrs: Attr[]): Promise<void> { return this.immediate(LevelInfo, name, attrs); }
  async track(name: string, ...attrs: Attr[]): Promise<void> { return this.event(name, ...attrs); }
  async audit(name: string, ...attrs: Attr[]): Promise<void> {
    const all: Attr[] = [StringAttr('audit.name', name), ...attrs];
    return this.immediate(LevelInfo, name, all);
  }
  async security(name: string, ...attrs: Attr[]): Promise<void> {
    const all: Attr[] = [StringAttr('security.name', name), ...attrs];
    return this.immediate(LevelWarn, name, all);
  }
  async metric(name: string, value: number, ...attrs: Attr[]): Promise<void> {
    const all: Attr[] = [Float64('value', value), ...attrs];
    return this.immediate(LevelInfo, `metric.${name}`, all);
  }
  async count(name: string, value: number, ...attrs: Attr[]): Promise<void> {
    const all: Attr[] = [Int64('count', value), ...attrs];
    return this.immediate(LevelInfo, `metric.${name}`, all);
  }
  async gauge(name: string, value: number, ...attrs: Attr[]): Promise<void> {
    const all: Attr[] = [Float64('gauge', value), ...attrs];
    return this.immediate(LevelInfo, `metric.${name}`, all);
  }
  async histogram(name: string, value: number, ...attrs: Attr[]): Promise<void> {
    const all: Attr[] = [Float64('observation', value), ...attrs];
    return this.immediate(LevelInfo, `metric.${name}`, all);
  }
  async breadcrumb(name: string, ...attrs: Attr[]): Promise<void> { return this.immediate(LevelDebug, name, attrs); }

  // Lifecycle outcome helpers
  async drop(ctx: Event, reason: string, ...attrs: Attr[]): Promise<string | null> {
    ctx.finish('dropped', StringAttr('drop_reason', reason), ...attrs);
    return this.emit(ctx);
  }
  async cancel(ctx: Event, reason: string, ...attrs: Attr[]): Promise<string | null> {
    ctx.finish('cancelled', StringAttr('cancel_reason', reason), ...attrs);
    return this.emit(ctx);
  }
  async abandon(ctx: Event, reason: string, ...attrs: Attr[]): Promise<string | null> {
    ctx.finish('abandoned', StringAttr('abandon_reason', reason), ...attrs);
    return this.emit(ctx);
  }
  async retry(ctx: Event, ...attrs: Attr[]): Promise<string | null> {
    ctx.finish('retried', ...attrs);
    return this.emit(ctx);
  }
  async partial(ctx: Event, ...attrs: Attr[]): Promise<string | null> {
    ctx.finish('partial', ...attrs);
    return this.emit(ctx);
  }

  // Clone / Link / Current / Bind / Wrap
  cloneEvent(ctx: Event): Event {
    return ctx.clone();
  }
  linkEvent(ctx: Event, target: string, ...attrs: Attr[]): Event {
    const ev = new Event({
      event: target,
      kind: ctx.kind,
      traceId: ctx.traceId,
      spanId: ctx.spanId,
      service: ctx.service,
    }, ctx.service, ctx.environment);
    if (attrs.length > 0) ev.enrich(...attrs);
    return ev;
  }
  currentEvent(): Event | undefined {
    return getEvent();
  }
  static bindEvent(params: Params, fn: (ctx: Event) => void | Promise<void>, finishAttrs: Attr[] = []): Promise<string | null> {
    return getDefault().runEvent(params, fn, finishAttrs);
  }
  static wrap(name: string, fn: () => void | Promise<void>): Promise<string | null> {
    return getDefault().runEvent({ event: name }, fn);
  }

  /** Flush the sink. */
  flush(): Promise<void> {
    const sink = this._resolvedSink;
    if (sink) return Promise.resolve(sink.flush()) as Promise<void>;
    return Promise.resolve();
  }

  /** Close the sink. */
  close(): Promise<void> {
    const sink = this._resolvedSink;
    if (sink) return Promise.resolve(sink.close()) as Promise<void>;
    return Promise.resolve();
  }

  /** Shutdown — alias for close(). */
  shutdown(): Promise<void> {
    return this.close();
  }

  /** Drain the sink queue — stop accepting new events and flush pending. */
  drain(): Promise<void> {
    const sink = this._resolvedSink;
    if (sink?.drain) return Promise.resolve(sink.drain()) as Promise<void>;
    return this.flush();
  }

  /** Pause event emission. */
  pause(): void {
    const sink = this._resolvedSink;
    if (sink?.pause) sink.pause();
  }

  /** Resume event emission. */
  resume(): void {
    const sink = this._resolvedSink;
    if (sink?.resume) sink.resume();
  }

  /** Return the current sink queue size (events pending flush). */
  queueSize(): number {
    const sink = this._resolvedSink;
    if (sink?.queueSize) return sink.queueSize();
    return 0;
  }

  /** Return SDK health status. */
  health(): boolean | Promise<boolean> {
    const sink = this._resolvedSink;
    if (sink?.health) return sink.health();
    return true;
  }

  /** Reconfigure this logger in-place (used by configure() to update the exported loza instance). */
  _reconfigure(cfg: Config): void {
    this.cfg = { ...cfg };
    this._resolvedSink = resolveSink(this.cfg);
  }
}

export function New(cfg?: Partial<Config>): Logger {
  return new Logger(cfg);
}

export function bindEvent(params: Params, fn: (ctx: Event) => void | Promise<void>, finishAttrs: Attr[] = []): Promise<string | null> {
  return Logger.bindEvent(params, fn, finishAttrs);
}

export function wrap(name: string, fn: () => void | Promise<void>): Promise<string | null> {
  return Logger.wrap(name, fn);
}

/**
 * Create an event from a framework HTTP request object.
 * Extracts method, path, route, headers, and creates an HTTP event.
 * Cross-SDK parity with Go's StartHTTPEventFromRequest.
 */
export function fromRequest(req: any, logger?: Logger): Event {
  const l = logger ?? getDefault();
  const method = req?.method ?? req?.httpMethod ?? 'GET';
  const path = req?.path ?? req?.url ?? req?.originalUrl ?? req?.pathname ?? '/';
  const route = req?.route?.path ?? req?.route ?? req?.urlPattern ?? path;
  const requestId = req?.headers?.['x-request-id'] ?? req?.requestId ?? '';
  const traceId = req?.headers?.['traceparent'] ?? req?.headers?.['x-trace-id'] ?? req?.traceId ?? '';
  const userAgent = req?.headers?.['user-agent'] ?? req?.userAgent ?? '';
  const referer = req?.headers?.['referer'] ?? req?.headers?.['referrer'] ?? '';
  const ev = l.startEvent({
    event: 'http.request',
    kind: 'http',
    method: method.toUpperCase(),
    path: path,
    route: route,
    level: 'info',
  });
  if (requestId) ev.enrich(StringAttr('request.id', requestId));
  if (traceId) ev.enrich(StringAttr('trace.id', traceId));
  if (userAgent) ev.enrich(StringAttr('http.user_agent', userAgent.slice(0, 512)));
  if (referer) ev.enrich(StringAttr('http.referer', referer.split('?')[0]));
  return ev;
}

/**
 * Run with an existing event context and auto finish/emit.
 * Cross-SDK parity with Go's Run().
 */
export async function run(ctx: Event, fn: () => void | Promise<void>, ...finishAttrs: Attr[]): Promise<string | null> {
  const l = getDefault();
  try {
    await fn();
    if (!ctx.getEventState().startsWith('finished')) {
      l.finish(ctx, 'success', ...finishAttrs);
    }
  } catch (err) {
    l.finishError(ctx, err instanceof Error ? err : new Error(`${err}`), ...finishAttrs);
  }
  return l.emit(ctx);
}

export function TryNew(cfg?: Partial<Config>): Logger {
  return new Logger(cfg);
}

export function Default(): Logger {
  return getDefault();
}

export const Configure = configure;

// --- Global logger ---

let _default: Logger | null = null;

/** Get or create the global default logger. */
export function getDefault(): Logger {
  if (!_default) _default = new Logger();
  return _default;
}

/** Configure the global default logger. Auto-creates HTTPBatchSink if collectorUrl is set. */
export function configure(cfg: Config): void {
  _default = new Logger(cfg);
}

/** Reset the global logger (for testing). */
export function reset(): void {
  _default = null;
}
