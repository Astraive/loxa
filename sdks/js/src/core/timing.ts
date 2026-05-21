/** Timing primitives: Process, Timer, Group, Stopwatch. */

export interface ProcessEntry {
  step: number;
  name: string;
  started_at_ms: number;
  ended_at_ms: number;
  duration_ms: number;
  status_code?: number;
  attrs?: Record<string, any>;
}

export interface GroupEntry {
  name: string;
  started_at_ms: number;
  ended_at_ms: number;
  duration_ms: number;
  status_code?: number;
  attrs?: Record<string, any>;
}

export interface TimerEntry {
  name: string;
  duration_ms: number;
  status_code?: number;
  attrs?: Record<string, any>;
}

function extractStatusCode(attrs: Record<string, any>): { code: number; rest: Record<string, any> } {
  let code = 0;
  const rest: Record<string, any> = {};
  for (const [k, v] of Object.entries(attrs)) {
    if (k === 'status_code' && typeof v === 'number') {
      code = v;
    } else {
      rest[k] = v;
    }
  }
  return { code, rest };
}

/** Handle for a named process step with automatic duration tracking. */
export class ProcessHandle {
  private _event: any;
  private _name: string;
  private _step: number;
  private _startedAt: number;
  private _startedAtMs: number;

  constructor(event: any, name: string, step: number, startedAt: number) {
    this._event = event;
    this._name = name;
    this._step = step;
    this._startedAt = startedAt;
    this._startedAtMs = startedAt - (this._event.startedAt || startedAt);
  }

  finish(...attrs: any[]): void {
    const now = Date.now();
    const endedMs = now - (this._event.startedAt || now);
    const extra: Record<string, any> = {};
    let statusCode = 0;
    for (const a of attrs) {
      if (a && a.key) {
        if (a.key === 'status_code' && typeof a.value === 'number') {
          statusCode = a.value;
        } else {
          extra[a.key] = a.value;
        }
      }
    }
    const entry: ProcessEntry = {
      step: this._step,
      name: this._name,
      started_at_ms: this._startedAtMs,
      ended_at_ms: endedMs,
      duration_ms: endedMs - this._startedAtMs,
    };
    if (statusCode) entry.status_code = statusCode;
    if (Object.keys(extra).length > 0) entry.attrs = extra;
    if (!this._event.processes) this._event.processes = [];
    this._event.processes.push(entry);
  }

  finishError(err: unknown, ...attrs: any[]): void {
    const extra = { error_message: String(err) };
    this.finish({ key: 'error_message', value: String(err) }, ...attrs);
  }

  duration(): number {
    return Date.now() - this._startedAt;
  }
}

/** Handle for a named timer with automatic duration tracking. */
export class TimerHandle {
  private _event: any;
  private _name: string;
  private _startedAt: number;

  constructor(event: any, name: string, startedAt: number) {
    this._event = event;
    this._name = name;
    this._startedAt = startedAt;
  }

  stop(...attrs: any[]): void {
    const durationMs = Date.now() - this._startedAt;
    const extra: Record<string, any> = {};
    let statusCode = 0;
    for (const a of attrs) {
      if (a && a.key) {
        if (a.key === 'status_code' && typeof a.value === 'number') {
          statusCode = a.value;
        } else {
          extra[a.key] = a.value;
        }
      }
    }
    const entry: TimerEntry = {
      name: this._name,
      duration_ms: durationMs,
    };
    if (statusCode) entry.status_code = statusCode;
    if (Object.keys(extra).length > 0) entry.attrs = extra;
    if (!this._event.timers) this._event.timers = [];
    this._event.timers.push(entry);
  }

  duration(): number {
    return Date.now() - this._startedAt;
  }
}

/** Handle for a named group phase with automatic duration tracking. */
export class GroupHandle {
  private _event: any;
  private _name: string;
  private _startedAt: number;
  private _startedAtMs: number;

  constructor(event: any, name: string, startedAt: number) {
    this._event = event;
    this._name = name;
    this._startedAt = startedAt;
    this._startedAtMs = startedAt - (this._event.startedAt || startedAt);
  }

  finish(...attrs: any[]): void {
    const now = Date.now();
    const endedMs = now - (this._event.startedAt || now);
    const extra: Record<string, any> = {};
    let statusCode = 0;
    for (const a of attrs) {
      if (a && a.key) {
        if (a.key === 'status_code' && typeof a.value === 'number') {
          statusCode = a.value;
        } else {
          extra[a.key] = a.value;
        }
      }
    }
    const entry: GroupEntry = {
      name: this._name,
      started_at_ms: this._startedAtMs,
      ended_at_ms: endedMs,
      duration_ms: endedMs - this._startedAtMs,
    };
    if (statusCode) entry.status_code = statusCode;
    if (Object.keys(extra).length > 0) entry.attrs = extra;
    if (!this._event.groups) this._event.groups = [];
    this._event.groups.push(entry);
  }

  duration(): number {
    return Date.now() - this._startedAt;
  }
}

/** Standalone elapsed-time measurer with no event reference. */
export class StopwatchHandle {
  private _startedAt: number;

  constructor() {
    this._startedAt = Date.now();
  }

  elapsed(): number {
    return Date.now() - this._startedAt;
  }
}

/** Create a new standalone stopwatch. */
export function stopwatch(): StopwatchHandle {
  return new StopwatchHandle();
}
