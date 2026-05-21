import { uuidv7 } from './uuidv7.ts';
import {
  EventClosedError,
  EventAlreadyFinishedError,
  extractError,
} from './errors.ts';
import type { ErrorInfo } from './errors.ts';
import { LevelInfo, levelName, parseLevel } from './level.ts';
import type { Level } from './level.ts';
import { ProcessHandle, TimerHandle, GroupHandle } from './timing.ts';
import type { ProcessEntry, GroupEntry, TimerEntry } from './timing.ts';

// --- Attr types ---

export type AttrKind = 'string' | 'number' | 'boolean' | 'null' | 'group' | 'any';

export interface Attr {
  key: string;
  kind: AttrKind;
  value: any;
  sensitive?: boolean;
  hashValue?: boolean;
  drop?: boolean;
}

export function String(key: string, value: string): Attr {
  return { key, kind: 'string', value };
}
export function Int(key: string, value: number): Attr {
  return { key, kind: 'number', value };
}
export function Float64(key: string, value: number): Attr {
  return { key, kind: 'number', value };
}
export function Bool(key: string, value: boolean): Attr {
  return { key, kind: 'boolean', value };
}
export function Null(key: string): Attr {
  return { key, kind: 'null', value: null };
}
export function Any(key: string, value: any): Attr {
  return { key, kind: 'any', value };
}
export function Group(key: string, attrs: Attr[]): Attr {
  return { key, kind: 'group', value: attrs };
}
export function SensitiveString(key: string, value: string): Attr {
  return { key, kind: 'string', value, sensitive: true };
}
export function HashString(key: string, value: string): Attr {
  return { key, kind: 'string', value, hashValue: true };
}
export function MarkSensitive(attr: Attr): Attr {
  return { ...attr, sensitive: true };
}
export function Int64(key: string, value: number): Attr {
  return { key, kind: 'number', value };
}
export function Uint64(key: string, value: number): Attr {
  return { key, kind: 'number', value };
}
export function Time(key: string, value: Date): Attr {
  return { key, kind: 'any', value: value.toISOString() };
}
export function Duration(key: string, value: number): Attr {
  return { key, kind: 'number', value };
}

// camelCase aliases for primitive constructors
export const string = String;
export const int = Int;
export const float64 = Float64;
export const bool = Bool;
export const null_ = Null;
export const any = Any;
export const group = Group;
export const sensitiveString = SensitiveString;
export const hashString = HashString;
export const markSensitive = MarkSensitive;
export const int64 = Int64;
export const uint64 = Uint64;
export const time = Time;
export const duration = Duration;

// Semantic shortcuts (canonical keys)
export function UserID(id: string): Attr { return String('user.id', id); }
export function TenantID(id: string): Attr { return String('tenant.id', id); }
export function WorkspaceID(id: string): Attr { return String('workspace.id', id); }
export function OrganizationID(id: string): Attr { return String('organization.id', id); }
export function SessionID(id: string): Attr { return String('session.id', id); }
export function RequestID(id: string): Attr { return String('request_id', id); }
export function TraceID(id: string): Attr { return String('trace_id', id); }
export function SpanID(id: string): Attr { return String('span_id', id); }
export function FeatureFlag(name: string, value: any): Attr { return Any(`feature.${name}`, value); }
export function FeatureFlagBool(name: string, value: boolean): Attr { return Bool(`feature.${name}`, value); }
export function Experiment(name: string, variant: string): Attr { return String(`experiment.${name}`, variant); }
export function OrderID(id: string): Attr { return String('order.id', id); }
export function CartID(id: string): Attr { return String('cart.id', id); }
export function ProductID(id: string): Attr { return String('product.id', id); }
export function CustomerID(id: string): Attr { return String('customer.id', id); }
export function Plan(plan: string): Attr { return String('customer.plan', plan); }
export function Currency(currency: string): Attr { return String('payment.currency', currency); }
export function Amount(amount: number): Attr { return Float64('payment.amount', amount); }
export function Country(country: string): Attr { return String('geo.country', country); }
export function Device(device: string): Attr { return String('device.name', device); }
export function Platform(platform: string): Attr { return String('device.platform', platform); }
export function AppVersion(version: string): Attr { return String('app.version', version); }
export function ErrorType(type: string): Attr { return String('error.type', type); }
export function ErrorCode(code: string): Attr { return String('error.code', code); }
export function ErrorMessage(msg: string): Attr { return String('error.message', msg); }
export function ErrorStack(stack: string): Attr { return String('error.stack', stack); }
export function Retryable(value: boolean): Attr { return Bool('error.retryable', value); }

// camelCase aliases for semantic shortcuts
export const userId = UserID;
export const tenantId = TenantID;
export const workspaceId = WorkspaceID;
export const organizationId = OrganizationID;
export const sessionId = SessionID;
export const requestId = RequestID;
export const traceId = TraceID;
export const spanId = SpanID;
export const featureFlag = FeatureFlag;
export const featureFlagBool = FeatureFlagBool;
export const experiment = Experiment;
export const orderId = OrderID;
export const cartId = CartID;
export const productId = ProductID;
export const customerId = CustomerID;
export const plan = Plan;
export const currency = Currency;
export const amount = Amount;
export const country = Country;
export const device = Device;
export const platform = Platform;
export const appVersion = AppVersion;
export const errorType = ErrorType;
export const errorCode = ErrorCode;
export const errorMessage = ErrorMessage;
export const errorStack = ErrorStack;
export const retryable = Retryable;

// --- Event states ---

export const EventStateCreated = 'created';
export const EventStateActive = 'active';
export const EventStateFinished = 'finished';
export const EventStateEmitting = 'emitting';
export const EventStateEmitted = 'emitted';
export const EventStateFailedValidation = 'failed_validation';
export const EventStateDeliveryFailed = 'delivery_failed';

export type EventState =
  | typeof EventStateCreated
  | typeof EventStateActive
  | typeof EventStateFinished
  | typeof EventStateEmitting
  | typeof EventStateEmitted
  | typeof EventStateFailedValidation
  | typeof EventStateDeliveryFailed;

// --- Checkpoint ---

export interface Checkpoint {
  name: string;
  at_ms: number;
  attrs?: Record<string, any>;
}

// --- Params ---

export interface Params {
  event?: string;
  name?: string;
  kind?: string;
  message?: string;
  level?: Level | string;
  service?: string;
  version?: string;
  environment?: string;
  region?: string;
  deploymentId?: string;
  host?: string;
  runtime?: string;
  method?: string;
  path?: string;
  route?: string;
  statusCode?: number;
  durationMs?: number;
  userId?: string;
  tenantId?: string;
  workspaceId?: string;
  organizationId?: string;
  sessionId?: string;
  requestId?: string;
  traceId?: string;
  spanId?: string;
  parentId?: string;
  outcome?: string;
  custom?: Attr[];
}

// --- Event ---

export class Event {
  schemaVersion = 'v1';
  eventVersion = 'v1';
  eventId: string;
  requestId: string;
  traceId: string;
  spanId: string;
  parentId = '';

  timestamp: string;
  service: string;
  event: string;
  kind: string;
  level: Level;
  message = '';
  outcome = '';

  version = '';
  environment = '';
  deploymentId = '';
  region = '';
  host = '';
  runtime = '';

  method = '';
  path = '';
  route = '';
  statusCode = 0;
  durationMs = 0;

  startedAt: number;
  finishedAt = 0;

  attrs: Record<string, any> = {};
  checkpoints: Checkpoint[] = [];
  processes: ProcessEntry[] = [];
  groups: GroupEntry[] = [];
  timers: TimerEntry[] = [];
  error: ErrorInfo | null = null;

  private _processStep = 0;

  private _state: EventState = EventStateCreated;
  private _emitted = false;
  private _sensitiveKeys = new Set<string>();
  private _hashKeys = new Set<string>();
  private _droppedKeys = new Set<string>();

  constructor(params: Params, service: string, environment: string) {
    this.eventId = uuidv7();
    this.requestId = params.requestId || uuidv7();
    this.traceId = params.traceId || '';
    this.spanId = params.spanId || '';
    this.parentId = params.parentId || '';

    this.startedAt = Date.now();
    this.timestamp = new Date(this.startedAt).toISOString();

    this.service = params.service || service;
    this.event = params.event || params.name || '';
    this.kind = params.kind || 'event';
    this.level = typeof params.level === 'string' ? parseLevel(params.level) : (params.level ?? LevelInfo);
    this.message = params.message || '';

    this.version = params.version || '';
    this.environment = params.environment || environment;
    this.deploymentId = params.deploymentId || '';
    this.region = params.region || '';
    this.host = params.host || '';
    this.runtime = params.runtime || 'node';

    this.method = params.method || '';
    this.path = params.path || '';
    this.route = params.route || '';
    this.statusCode = params.statusCode || 0;
    this.durationMs = params.durationMs || 0;

    if (params.userId) this.attrs['user.id'] = params.userId;
    if (params.tenantId) this.attrs['tenant.id'] = params.tenantId;
    if (params.workspaceId) this.attrs['workspace.id'] = params.workspaceId;
    if (params.organizationId) this.attrs['organization.id'] = params.organizationId;
    if (params.sessionId) this.attrs['session.id'] = params.sessionId;

    if (params.custom) {
      for (const attr of params.custom) {
        this.applyAttr(attr);
      }
    }

    if (params.outcome) this.outcome = params.outcome;
  }

  get state(): EventState { return this._state; }
  get emitted(): boolean { return this._emitted; }
  get sensitiveKeys(): ReadonlySet<string> { return this._sensitiveKeys; }
  get hashKeys(): ReadonlySet<string> { return this._hashKeys; }
  get droppedKeys(): ReadonlySet<string> { return this._droppedKeys; }

  /** Ensure the event can be mutated. */
  private ensureMutable(): void {
    if (this._emitted || this._state === EventStateEmitted || this._state === EventStateFailedValidation) {
      throw new EventClosedError();
    }
  }

  /** Transition to active state if currently created. */
  private markActive(): void {
    if (this._state === EventStateCreated) {
      this._state = EventStateActive;
    }
  }

  /** Apply a single attr, respecting sensitive/hash/drop flags. */
  private applyAttr(attr: Attr): void {
    if (attr.drop) {
      this._droppedKeys.add(attr.key);
      delete this.attrs[attr.key];
      return;
    }
    if (attr.sensitive) this._sensitiveKeys.add(attr.key);
    if (attr.hashValue) this._hashKeys.add(attr.key);
    this.attrs[attr.key] = attr.value;
  }

  /** Enrich the event with attributes. */
  enrich(...attrs: Attr[]): void {
    this.ensureMutable();
    this.markActive();
    for (const attr of attrs) {
      this.applyAttr(attr);
    }
  }

  /** Alias for enrich — the v1-preferred name. */
  append(...attrs: Attr[]): void {
    this.enrich(...attrs);
  }

  /** Set a single attr by key (replaces if exists). */
  set(key: string, value: any): void {
    this.ensureMutable();
    this.markActive();
    this.attrs[key] = value;
  }

  /** Merge a plain object's entries into attrs. */
  merge(obj: Record<string, any>): void {
    this.ensureMutable();
    this.markActive();
    Object.assign(this.attrs, obj);
  }

  /** Remove an attr by key. */
  delete(key: string): void {
    this.ensureMutable();
    delete this.attrs[key];
  }

  /** Return the value for a key, or undefined. */
  get(key: string): any {
    return this.attrs[key];
  }

  /** Return all attrs whose key starts with prefix., with prefix stripped. */
  getGroup(prefix: string): Record<string, any> {
    const result: Record<string, any> = {};
    const p = prefix + '.';
    for (const [k, v] of Object.entries(this.attrs)) {
      if (k.startsWith(p)) {
        result[k.slice(p.length)] = v;
      }
    }
    return result;
  }

  /** Record a checkpoint (breadcrumb). */
  checkpoint(name: string, attrs?: Record<string, any>): void {
    this.ensureMutable();
    this.checkpoints.push({
      name,
      at_ms: Date.now() - this.startedAt,
      attrs,
    });
  }

  /** Start a named process step and return a handle to finish it. */
  startProcess(name: string, ...attrs: Attr[]): ProcessHandle {
    this.ensureMutable();
    this._processStep++;
    return new ProcessHandle(this, name, this._processStep, Date.now());
  }

  /** Start a named timer and return a handle to stop it. */
  startTimer(name: string, ...attrs: Attr[]): TimerHandle {
    this.ensureMutable();
    return new TimerHandle(this, name, Date.now());
  }

  /** Start a named group phase and return a handle to finish it. */
  startGroup(name: string, ...attrs: Attr[]): GroupHandle {
    this.ensureMutable();
    return new GroupHandle(this, name, Date.now());
  }

  /** Finish the event with an outcome. */
  finish(outcome: string, ...attrs: Attr[]): void {
    this.ensureMutable();
    if (this._state === EventStateFinished) {
      throw new EventAlreadyFinishedError();
    }
    this.outcome = outcome;
    this.finishedAt = Date.now();
    this.durationMs = this.finishedAt - this.startedAt;
    this._state = EventStateFinished;
    for (const attr of attrs) {
      this.applyAttr(attr);
    }
  }

  /** Finish the event with an error. */
  finishError(err: unknown, ...attrs: Attr[]): void {
    this.error = extractError(err);
    this.finish('error', ...attrs);
  }

  /** Mark the event as emitted. Returns false if already emitted. */
  markEmitted(): boolean {
    if (this._emitted) return false;
    this._emitted = true;
    this._state = EventStateEmitting;
    return true;
  }

  /** Mark the event as successfully emitted. */
  markEmittedDone(): void {
    this._state = EventStateEmitted;
  }

  /** Mark the event as failed validation. */
  markFailedValidation(): void {
    this._state = EventStateFailedValidation;
  }

  /** Mark the event as delivery failed. */
  markDeliveryFailed(): void {
    this._state = EventStateDeliveryFailed;
  }

  /** Get the current event state as a string. */
  getEventState(): string {
    return this._state;
  }

  /** Clone the event (deep copy). */
  clone(): Event {
    const ev = new Event({ event: this.event, service: this.service }, this.service, this.environment);
    Object.assign(ev, this);
    ev.attrs = { ...this.attrs };
    ev.checkpoints = [...this.checkpoints];
    ev.error = this.error ? { ...this.error } : null;
    ev._state = this._state;
    ev._emitted = this._emitted;
    ev._sensitiveKeys = new Set(this._sensitiveKeys);
    ev._hashKeys = new Set(this._hashKeys);
    ev._droppedKeys = new Set(this._droppedKeys);
    return ev;
  }
}
