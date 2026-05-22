import { AsyncLocalStorage } from 'node:async_hooks';
import { Event } from './event.ts';

const storage = new AsyncLocalStorage<Event>();

/** Store an event in the current async context. */
export function storeEvent(event: Event): void {
  storage.enterWith(event);
}

/** Get the event from the current async context, or undefined. */
export function getEvent(): Event | undefined {
  return storage.getStore();
}

/** Check if there's an event in the current context. */
export function hasEvent(): boolean {
  return storage.getStore() !== undefined;
}

/** Get the event ID from the current context. */
export function eventId(): string {
  return storage.getStore()?.eventId ?? '';
}

/** Run a function with a specific event in context. */
export function runWithEvent<T>(event: Event, fn: () => T): T {
  return storage.run(event, fn);
}

/** Get the request ID from the current context. */
export function requestIdFromContext(): string {
  return storage.getStore()?.requestId ?? '';
}

/** Get the trace ID from the current context. */
export function traceIdFromContext(): string {
  return storage.getStore()?.traceId ?? '';
}
