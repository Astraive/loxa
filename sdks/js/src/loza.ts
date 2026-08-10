/**
 * loza-js default facade — the `loza` default Logger instance + module-level functions.
 *
 * Usage:
 *   import { loza, createLoza } from "@astraive/loza";
 *   loza.configure(loza.production("checkout").withSink(loza.httpBatchSink({...})));
 *   const ctx = loza.startEvent({ event: "checkout.request" });
 *   loza.append(ctx, loza.userId("u_123"));
 *   loza.finish(ctx, "success");
 *   await loza.emit(ctx);
 *
 *   // Custom instance
 *   const logger = createLoza({ service: "checkout-api" });
 *   logger.info("started");
 */

import { reset, Logger, fromRequest } from './core/logger.ts';
import type { Config } from './config/config.ts';
import type { Event, Params, Attr } from './core/event.ts';
import type { ProcessHandle, TimerHandle, GroupHandle } from './core/timing.ts';

/** The default global Logger instance. Use `loza.info(...)`, `loza.enrich(...)`, etc. */
export const loza: Logger = new Logger();

/** Configure the default global logger. Updates the exported `loza` instance in-place. */
export function configure(cfg: Config): void {
  loza._reconfigure(cfg);
}

export function defaultLogger(): Logger { return loza; }
export { reset };

/** Create a new Logger instance with the given config. */
export function createLoza(cfg?: Partial<Config>): Logger { return new Logger(cfg); }

/** Create a same-config child logger with loza.alias metadata. */
export function alias(name: string): Logger { return loza.alias(name); }

// --- Event lifecycle ---

export function startEvent(params: Params): Event { return loza.startEvent(params); }
export function startHttpEvent(params: Params): Event { return loza.startHTTPEvent(params); }
export function startJobEvent(params: Params): Event { return loza.startJobEvent(params); }
export function startQueueEvent(params: Params): Event { return loza.startQueueEvent(params); }
export function startCliEvent(params: Params): Event { return loza.startCLIEvent(params); }
export function startCronEvent(params: Params): Event { return loza.startCronEvent(params); }

// --- Event mutation ---

export function append(ctx: Event, ...attrs: Attr[]): void { loza.append(ctx, ...attrs); }
export function enrich(ctx: Event, ...attrs: Attr[]): void { loza.enrich(ctx, ...attrs); }
export function set(ctx: Event, key: string, value: any): void { loza.set(ctx, key, value); }
export function merge(ctx: Event, obj: Record<string, any>): void { loza.merge(ctx, obj); }
export function del(ctx: Event, key: string): void { loza.delete(ctx, key); }
export function get(ctx: Event, key: string): any { return loza.get(ctx, key); }
export function getGroup(ctx: Event, prefix: string): Record<string, any> { return loza.getGroup(ctx, prefix); }

// --- Event lifecycle ---

export function checkpoint(ctx: Event, name: string, attrs?: Record<string, any>): void { loza.checkpoint(ctx, name, attrs); }
export function process(ctx: Event, name: string, ...attrs: Attr[]): ProcessHandle { return ctx.startProcess(name, ...attrs); }
export function startProcess(ctx: Event, name: string, ...attrs: Attr[]): ProcessHandle { return process(ctx, name, ...attrs); }
export function finishProcess(handle: ProcessHandle, ...attrs: Attr[]): void { handle.finish(...attrs); }
export function finishProcessError(handle: ProcessHandle, err: unknown, ...attrs: Attr[]): void { handle.finishError(err, ...attrs); }
export function group(ctx: Event, name: string, ...attrs: Attr[]): GroupHandle { return ctx.startGroup(name, ...attrs); }
export function startTimer(ctx: Event, name: string, ...attrs: Attr[]): TimerHandle { return ctx.startTimer(name, ...attrs); }
export function timer(ctx: Event, name: string, ...attrs: Attr[]): TimerHandle { return startTimer(ctx, name, ...attrs); }
export function stopTimer(handle: TimerHandle, ...attrs: Attr[]): void { handle.stop(...attrs); }
export function startGroup(ctx: Event, name: string, ...attrs: Attr[]): GroupHandle { return ctx.startGroup(name, ...attrs); }
export function finishGroup(handle: GroupHandle, ...attrs: Attr[]): void { handle.finish(...attrs); }
export function finishGroupError(handle: GroupHandle, err: unknown, ...attrs: Attr[]): void { handle.finishError(err, ...attrs); }
export { stopwatch, ProcessHandle, TimerHandle, GroupHandle, StopwatchHandle } from './core/timing.ts';
export function finish(ctx: Event, outcome: string, ...attrs: Attr[]): void { loza.finish(ctx, outcome, ...attrs); }
export function finishError(ctx: Event, err: unknown, ...attrs: Attr[]): void { loza.finishError(ctx, err, ...attrs); }
export async function emit(ctx: Event): Promise<string | null> { return loza.emit(ctx); }
export async function runEvent(params: Params, fn: (ctx: Event) => void | Promise<void>, finishAttrs?: Attr[]): Promise<string | null> {
  return loza.runEvent(params, fn, finishAttrs);
}
export async function run(ctx: Event, fn: (ctx: Event) => void | Promise<void>, finishAttrs?: Attr[]): Promise<string | null> {
  return loza.run(ctx, fn, finishAttrs);
}
export { fromRequest } from './core/logger.ts';

// --- Lifecycle management ---

export async function flush(): Promise<void> { return loza.flush(); }
export async function shutdown(): Promise<void> { return loza.shutdown(); }
export async function drain(): Promise<void> { return loza.drain(); }
export function pause(): void { loza.pause(); }
export function resume(): void { loza.resume(); }
export function queueSize(): number { return loza.queueSize(); }
export function health(): boolean | Promise<boolean> { return loza.health(); }

// --- Immediate log helpers ---

export async function debug(message: string, ...attrs: Attr[]): Promise<void> { return loza.debug(message, ...attrs); }
export async function info(message: string, ...attrs: Attr[]): Promise<void> { return loza.info(message, ...attrs); }
export async function notice(message: string, ...attrs: Attr[]): Promise<void> { return loza.notice(message, ...attrs); }
export async function warn(message: string, ...attrs: Attr[]): Promise<void> { return loza.warn(message, ...attrs); }
export async function error(message: string, ...attrs: Attr[]): Promise<void> { return loza.error(message, ...attrs); }
export async function fatal(message: string, ...attrs: Attr[]): Promise<void> { return loza.fatal(message, ...attrs); }

// --- Logging helpers ---

export async function event(name: string, ...attrs: Attr[]): Promise<void> { return loza.event(name, ...attrs); }
export async function track(name: string, ...attrs: Attr[]): Promise<void> { return loza.track(name, ...attrs); }
export async function audit(name: string, ...attrs: Attr[]): Promise<void> { return loza.audit(name, ...attrs); }
export async function security(name: string, ...attrs: Attr[]): Promise<void> { return loza.security(name, ...attrs); }
export async function metric(name: string, value: number, ...attrs: Attr[]): Promise<void> { return loza.metric(name, value, ...attrs); }
export async function count(name: string, value: number, ...attrs: Attr[]): Promise<void> { return loza.count(name, value, ...attrs); }
export async function gauge(name: string, value: number, ...attrs: Attr[]): Promise<void> { return loza.gauge(name, value, ...attrs); }
export async function histogram(name: string, value: number, ...attrs: Attr[]): Promise<void> { return loza.histogram(name, value, ...attrs); }
export async function breadcrumb(name: string, ...attrs: Attr[]): Promise<void> { return loza.breadcrumb(name, ...attrs); }

// --- Lifecycle outcome helpers ---

export async function drop(ctx: Event, reason: string, ...attrs: Attr[]): Promise<string | null> { return loza.drop(ctx, reason, ...attrs); }
export async function cancel(ctx: Event, reason: string, ...attrs: Attr[]): Promise<string | null> { return loza.cancel(ctx, reason, ...attrs); }
export async function abandon(ctx: Event, reason: string, ...attrs: Attr[]): Promise<string | null> { return loza.abandon(ctx, reason, ...attrs); }
export async function retry(ctx: Event, ...attrs: Attr[]): Promise<string | null> { return loza.retry(ctx, ...attrs); }
export async function partial(ctx: Event, ...attrs: Attr[]): Promise<string | null> { return loza.partial(ctx, ...attrs); }

// --- Clone / Link / Current ---

export function cloneEvent(ctx: Event): Event { return loza.cloneEvent(ctx); }
export function linkEvent(ctx: Event, target: string, ...attrs: Attr[]): Event { return loza.linkEvent(ctx, target, ...attrs); }
export function currentEvent(): Event | undefined { return loza.currentEvent(); }

// --- Sanitize ---
export { sanitizeEvent } from './core/sanitize.ts';

// --- Testkit ---
export {
  testkit, testLogger, capture, events, lastEvent, clearEvents,
  assertEvent, assertAttr, assertRedacted, assertHasCheckpoint, expectEvent,
  expectAttr, snapshotEvent, goldenTest, conformanceSuite, MockSink,
  FakeClock, setClock, setIdGenerator, resetForTest,
} from './testkit/helpers.ts';
export type { TestLoggerResult } from './testkit/helpers.ts';

// --- SecurityLimiter ---
export { SecurityLimiter } from './config/security.ts';
export type { SecurityConfig as SecurityLimiterConfig } from './config/security.ts';

// Loza class is intentionally not exported. Use createLoza() or alias() instead.
