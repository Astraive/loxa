/**
 * loxa-js default facade — module-level functions that delegate to the global default Logger.
 *
 * Usage:
 *   import * as loxa from "loxa-js";
 *   loxa.configure(loxa.production("checkout").withSink(loxa.httpBatchSink({...})));
 *   const ctx = loxa.startEvent({ event: "checkout.request" });
 *   loxa.append(ctx, loxa.userId("u_123"));
 *   loxa.finish(ctx, "success");
 *   await loxa.emit(ctx);
 */

import { getDefault, configure, reset } from './core/logger.ts';
import type { Logger } from './core/logger.ts';
import type { Event, Params, Attr } from './core/event.ts';
import type { ProcessHandle, TimerHandle, GroupHandle } from './core/timing.ts';

export function defaultLogger(): Logger { return getDefault(); }
export { configure, reset };

// --- Event lifecycle ---

export function startEvent(params: Params): Event { return getDefault().startEvent(params); }
export function startHttpEvent(params: Params): Event { return getDefault().startHTTPEvent(params); }
export function startJobEvent(params: Params): Event { return getDefault().startJobEvent(params); }
export function startQueueEvent(params: Params): Event { return getDefault().startQueueEvent(params); }
export function startCliEvent(params: Params): Event { return getDefault().startCLIEvent(params); }
export function startCronEvent(params: Params): Event { return getDefault().startCronEvent(params); }

// --- Event mutation ---

export function append(ctx: Event, ...attrs: Attr[]): void { getDefault().append(ctx, ...attrs); }
export function enrich(ctx: Event, ...attrs: Attr[]): void { getDefault().enrich(ctx, ...attrs); }
export function set(ctx: Event, key: string, value: any): void { getDefault().set(ctx, key, value); }
export function merge(ctx: Event, obj: Record<string, any>): void { getDefault().merge(ctx, obj); }
export function del(ctx: Event, key: string): void { getDefault().delete(ctx, key); }
export function get(ctx: Event, key: string): any { return getDefault().get(ctx, key); }
export function getGroup(ctx: Event, prefix: string): Record<string, any> { return getDefault().getGroup(ctx, prefix); }

// --- Event lifecycle ---

export function checkpoint(ctx: Event, name: string, attrs?: Record<string, any>): void { getDefault().checkpoint(ctx, name, attrs); }
export function process(ctx: Event, name: string, ...attrs: Attr[]): ProcessHandle { return ctx.startProcess(name, ...attrs); }
export function startTimer(ctx: Event, name: string, ...attrs: Attr[]): TimerHandle { return ctx.startTimer(name, ...attrs); }
export function startGroup(ctx: Event, name: string, ...attrs: Attr[]): GroupHandle { return ctx.startGroup(name, ...attrs); }
export { stopwatch, ProcessHandle, TimerHandle, GroupHandle, StopwatchHandle } from './core/timing.ts';
export function finish(ctx: Event, outcome: string, ...attrs: Attr[]): void { getDefault().finish(ctx, outcome, ...attrs); }
export function finishError(ctx: Event, err: unknown, ...attrs: Attr[]): void { getDefault().finishError(ctx, err, ...attrs); }
export async function emit(ctx: Event): Promise<string | null> { return getDefault().emit(ctx); }
export async function runEvent(params: Params, fn: (ctx: Event) => void | Promise<void>, finishAttrs?: Attr[]): Promise<string | null> {
  return getDefault().runEvent(params, fn, finishAttrs);
}

// --- Lifecycle management ---

export async function flush(): Promise<void> { return getDefault().flush(); }
export async function shutdown(): Promise<void> { return getDefault().shutdown(); }

// --- Immediate log helpers ---

export async function debug(message: string, ...attrs: Attr[]): Promise<void> { return getDefault().debug(message, ...attrs); }
export async function info(message: string, ...attrs: Attr[]): Promise<void> { return getDefault().info(message, ...attrs); }
export async function warn(message: string, ...attrs: Attr[]): Promise<void> { return getDefault().warn(message, ...attrs); }
export async function error(message: string, ...attrs: Attr[]): Promise<void> { return getDefault().error(message, ...attrs); }
export async function fatal(message: string, ...attrs: Attr[]): Promise<void> { return getDefault().fatal(message, ...attrs); }

// --- Testkit ---
export { testLogger, capture, assertEvent, assertAttr, assertRedacted, assertHasCheckpoint } from './testkit/helpers.ts';
export type { TestLoggerResult } from './testkit/helpers.ts';

// --- SecurityLimiter ---
export { SecurityLimiter } from './config/security.ts';
export type { SecurityConfig as SecurityLimiterConfig } from './config/security.ts';
