import {
  Event, EventStateEmitting, EventStateEmitted,
} from './event.ts';
import type { Attr, Params } from './event.ts';
import { defaultConfig, withOptions, dev, production, test } from '../config/config.ts';
import type { Config, ConfigOptions } from '../config/config.ts';
import type { Sink } from '../sinks/sink.ts';
import { HTTPBatchSink } from '../sinks/standard-sinks.ts';
import { encodeJSON } from '../jsonenc/encoder.ts';
import { storeEvent, getEvent, runWithEvent } from './context.ts';
import { LevelDebug, LevelInfo, LevelWarn, LevelError, LevelFatal, parseLevel } from './level.ts';
import type { Level } from './level.ts';
import { sanitizeEvent } from './sanitize.ts';
import { EventView } from './event-view.ts';

/** Resolve the effective sink: explicit sink > HTTPBatchSink from collectorUrl > null. */
function resolveSink(cfg: Config): Sink | null {
  if (cfg.sink) return cfg.sink;
  if (cfg.collectorUrl) {
    return new HTTPBatchSink({
      endpoint: cfg.collectorUrl,
      apiKey: cfg.apiKey,
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

/** The core LOXA logger. */
export class Logger {
  private cfg: Config;
  private _resolvedSink: Sink | null;

  constructor(cfg?: Partial<Config>) {
    this.cfg = { ...defaultConfig(), ...cfg };
    this._resolvedSink = resolveSink(this.cfg);
  }

  /** Create a child logger with modified config. */
  child(opts: ConfigOptions): Logger {
    return new Logger(withOptions(this.cfg, opts));
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
      } catch {
        ctx.markDeliveryFailed();
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
  warn(message: string, ...attrs: Attr[]): Promise<void> { return this.immediate(LevelWarn, message, attrs); }
  error(message: string, ...attrs: Attr[]): Promise<void> { return this.immediate(LevelError, message, attrs); }
  fatal(message: string, ...attrs: Attr[]): Promise<void> { return this.immediate(LevelFatal, message, attrs); }

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
}

export function New(cfg?: Partial<Config>): Logger {
  return new Logger(cfg);
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
