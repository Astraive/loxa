/**
 * loxa-js default facade — the `loxa` default Logger instance + module-level functions.
 *
 * Usage:
 *   import { loxa, createLoxa } from "loxa-js";
 *   loxa.configure(loxa.production("checkout").withSink(loxa.httpBatchSink({...})));
 *   const ctx = loxa.startEvent({ event: "checkout.request" });
 *   loxa.append(ctx, loxa.userId("u_123"));
 *   loxa.finish(ctx, "success");
 *   await loxa.emit(ctx);
 *
 *   // Custom instance
 *   const logger = createLoxa({ service: "checkout-api" });
 *   logger.info("started");
 */

import { reset, Logger } from './core/logger.ts';
import type { Config } from './config/config.ts';
import type { Event, Params, Attr } from './core/event.ts';
import type { ProcessHandle, TimerHandle, GroupHandle } from './core/timing.ts';

/** The default global Logger instance. Use `loxa.info(...)`, `loxa.enrich(...)`, etc. */
export const loxa: Logger = new Logger();

/** Configure the default global logger. Updates the exported `loxa` instance in-place. */
export function configure(cfg: Config): void {
  loxa._reconfigure(cfg);
}

export function defaultLogger(): Logger { return loxa; }
export { reset };

/** Create a new Logger instance with the given config. */
export function createLoxa(cfg?: Partial<Config>): Logger { return new Logger(cfg); }

/** Create a new Logger with the same config as default but a different service name. */
export function alias(service: string): Logger { return loxa.alias(service); }

// --- Event lifecycle ---

export function startEvent(params: Params): Event { return loxa.startEvent(params); }
export function startHttpEvent(params: Params): Event { return loxa.startHTTPEvent(params); }
export function startJobEvent(params: Params): Event { return loxa.startJobEvent(params); }
export function startQueueEvent(params: Params): Event { return loxa.startQueueEvent(params); }
export function startCliEvent(params: Params): Event { return loxa.startCLIEvent(params); }
export function startCronEvent(params: Params): Event { return loxa.startCronEvent(params); }

// --- Event mutation ---

export function append(ctx: Event, ...attrs: Attr[]): void { loxa.append(ctx, ...attrs); }
export function enrich(ctx: Event, ...attrs: Attr[]): void { loxa.enrich(ctx, ...attrs); }
export function set(ctx: Event, key: string, value: any): void { loxa.set(ctx, key, value); }
export function merge(ctx: Event, obj: Record<string, any>): void { loxa.merge(ctx, obj); }
export function del(ctx: Event, key: string): void { loxa.delete(ctx, key); }
export function get(ctx: Event, key: string): any { return loxa.get(ctx, key); }
export function getGroup(ctx: Event, prefix: string): Record<string, any> { return loxa.getGroup(ctx, prefix); }

// --- Event lifecycle ---

export function checkpoint(ctx: Event, name: string, attrs?: Record<string, any>): void { loxa.checkpoint(ctx, name, attrs); }
export function process(ctx: Event, name: string, ...attrs: Attr[]): ProcessHandle { return ctx.startProcess(name, ...attrs); }
export function startTimer(ctx: Event, name: string, ...attrs: Attr[]): TimerHandle { return ctx.startTimer(name, ...attrs); }
export function startGroup(ctx: Event, name: string, ...attrs: Attr[]): GroupHandle { return ctx.startGroup(name, ...attrs); }
export { stopwatch, ProcessHandle, TimerHandle, GroupHandle, StopwatchHandle } from './core/timing.ts';
export function finish(ctx: Event, outcome: string, ...attrs: Attr[]): void { loxa.finish(ctx, outcome, ...attrs); }
export function finishError(ctx: Event, err: unknown, ...attrs: Attr[]): void { loxa.finishError(ctx, err, ...attrs); }
export async function emit(ctx: Event): Promise<string | null> { return loxa.emit(ctx); }
export async function runEvent(params: Params, fn: (ctx: Event) => void | Promise<void>, finishAttrs?: Attr[]): Promise<string | null> {
  return loxa.runEvent(params, fn, finishAttrs);
}

// --- Lifecycle management ---

export async function flush(): Promise<void> { return loxa.flush(); }
export async function shutdown(): Promise<void> { return loxa.shutdown(); }

// --- Immediate log helpers ---

export async function debug(message: string, ...attrs: Attr[]): Promise<void> { return loxa.debug(message, ...attrs); }
export async function info(message: string, ...attrs: Attr[]): Promise<void> { return loxa.info(message, ...attrs); }
export async function warn(message: string, ...attrs: Attr[]): Promise<void> { return loxa.warn(message, ...attrs); }
export async function error(message: string, ...attrs: Attr[]): Promise<void> { return loxa.error(message, ...attrs); }
export async function fatal(message: string, ...attrs: Attr[]): Promise<void> { return loxa.fatal(message, ...attrs); }

// --- Testkit ---
export { testLogger, capture, assertEvent, assertAttr, assertRedacted, assertHasCheckpoint } from './testkit/helpers.ts';
export type { TestLoggerResult } from './testkit/helpers.ts';

// --- SecurityLimiter ---
export { SecurityLimiter } from './config/security.ts';
export type { SecurityConfig as SecurityLimiterConfig } from './config/security.ts';
