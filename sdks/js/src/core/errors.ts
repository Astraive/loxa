/** Base error for all LOZA SDK errors. */
export class LozaError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'LozaError';
  }
}

/** Thrown when emit() is called twice on the same event. */
export class DuplicateEmitError extends LozaError {
  constructor() {
    super('event already emitted');
    this.name = 'DuplicateEmitError';
  }
}

/** Thrown when mutating an event that has been emitted or closed. */
export class EventClosedError extends LozaError {
  constructor() {
    super('event is closed');
    this.name = 'EventClosedError';
  }
}

/** Thrown when finish() is called twice. */
export class EventAlreadyFinishedError extends LozaError {
  constructor() {
    super('event already finished');
    this.name = 'EventAlreadyFinishedError';
  }
}

/** Thrown when strict validation fails. */
export class EventValidationError extends LozaError {
  constructor(message: string) {
    super(message);
    this.name = 'EventValidationError';
  }
}

/** Error info attached to an event. */
export interface ErrorInfo {
  type: string;
  message: string;
  stack?: string;
  retryable?: boolean;
}

/** Extract error info from a thrown value. */
export function extractError(err: unknown): ErrorInfo {
  if (err instanceof Error) {
    return {
      type: err.name || 'Error',
      message: err.message,
      stack: err.stack,
    };
  }
  return { type: 'Error', message: String(err) };
}
