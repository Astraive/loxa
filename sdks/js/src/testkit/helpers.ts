import { Logger } from '../core/logger.ts';
import { MemorySink } from '../sinks/standard-sinks.ts';
import type { Params } from '../core/event.ts';

export interface TestLoggerResult {
  logger: Logger;
  sink: MemorySink;
}

/** Creates a logger configured for tests plus its backing memory sink. */
export function testLogger(config?: Partial<Params>): TestLoggerResult {
  const sink = new MemorySink();
  const logger = new Logger({
    service: config?.service || 'test',
    environment: 'test',
    sink,
  });
  return { logger, sink };
}

/** Runs fn with a temporary memory sink and returns captured event JSON strings. */
export async function capture(fn: (logger: Logger) => void | Promise<void>): Promise<string[]> {
  const sink = new MemorySink();
  const logger = new Logger({
    service: 'test',
    environment: 'test',
    sink,
  });
  await fn(logger);
  await logger.flush();
  return sink.getEvents();
}

/** Asserts that a decoded event JSON contains the expected key-value pair. */
export function assertEvent(eventJson: string, key: string, expected: unknown): void {
  const parsed = JSON.parse(eventJson);
  const actual = getNestedValue(parsed, key);
  if (actual === undefined) {
    throw new Error(`assertEvent: key "${key}" not found in event`);
  }
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error(`assertEvent: key "${key}" expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
  }
}

/** Asserts that a decoded event JSON has a specific attribute value. */
export function assertAttr(eventJson: string, key: string, expected: unknown): void {
  assertEvent(eventJson, key, expected);
}

/** Asserts that a field is redacted. */
export function assertRedacted(eventJson: string, key: string): void {
  const parsed = JSON.parse(eventJson);
  const actual = getNestedValue(parsed, key);
  if (actual !== '[REDACTED]') {
    throw new Error(`assertRedacted: key "${key}" expected "[REDACTED]", got ${JSON.stringify(actual)}`);
  }
}

/** Asserts that a checkpoint exists in the event. */
export function assertHasCheckpoint(eventJson: string, name: string): void {
  const parsed = JSON.parse(eventJson);
  const checkpoints = parsed.checkpoints || [];
  const found = checkpoints.some((cp: any) => cp.name === name);
  if (!found) {
    throw new Error(`assertHasCheckpoint: checkpoint "${name}" not found`);
  }
}

function getNestedValue(obj: any, path: string): any {
  const parts = path.split('.');
  let current = obj;
  for (const part of parts) {
    if (current == null || typeof current !== 'object') return undefined;
    current = current[part];
  }
  return current;
}
