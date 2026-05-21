import { createHash } from 'node:crypto';
import type { Event } from './event.ts';

/**
 * Sanitize an event for safe schema encoding.
 * Clones the event, applies sensitive→[REDACTED], hashValue→sha256, drop→delete.
 * The original event is never mutated.
 */
export function sanitizeEvent(event: Event): Event {
  const clone = event.clone();

  // Apply sensitive keys → redact values
  for (const key of clone.sensitiveKeys) {
    if (key in clone.attrs) {
      clone.attrs[key] = '[REDACTED]';
    }
  }

  // Apply hash keys → sha256 hash values
  for (const key of clone.hashKeys) {
    if (key in clone.attrs) {
      const val = clone.attrs[key];
      if (typeof val === 'string') {
        clone.attrs[key] = createHash('sha256').update(val).digest('hex');
      }
    }
  }

  // Remove dropped keys
  for (const key of clone.droppedKeys) {
    delete clone.attrs[key];
  }

  return clone;
}
