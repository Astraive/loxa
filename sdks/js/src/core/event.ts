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
export const float = Float64;
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

// Domain helper pack — ID fields
export function PaymentID(id: string): Attr { return String('payment.id', id); }
export function SubscriptionID(id: string): Attr { return String('subscription.id', id); }
export function InvoiceID(id: string): Attr { return String('invoice.id', id); }
export function JobID(id: string): Attr { return String('job.id', id); }
export function MessageID(id: string): Attr { return String('message.id', id); }
export function CorrelationID(id: string): Attr { return String('correlation.id', id); }
export function CommitSha(sha: string): Attr { return String('deployment.commit_sha', sha); }
export function Release(ver: string): Attr { return String('release', ver); }

// Money helper — stores {key.amount_cents, key.currency}
export function Money(key: string, amountCents: number, currency: string): Attr {
  return Group(key, [Int64(`${key}.amount_cents`, amountCents), String(`${key}.currency`, currency)]);
}

// Percent/bytes helpers
export function Percent(key: string, value: number): Attr { return Float64(key, value); }
export function Bytes(key: string, value: number): Attr { return Int64(key, value); }

// HTTP/status helpers
export function HttpStatus(key: string, code: number): Attr { return Int(key, code); }
export function StatusCode(key: string, code: number): Attr { return Int(key, code); }
export function ErrorCodeExt(key: string, code: string): Attr { return String(key, code); }

// Bucket / tags / masked / url / hash helpers
export function Bucket(key: string, bucket: string): Attr { return String(key, bucket); }
export function Tags(key: string, ...values: string[]): Attr { return Any(key, values); }
export function Masked(key: string, value: string): Attr { return String(key, value); } // sensitive via callers
export function Url(key: string, value: string): Attr { return HashString(key, value); }
export function EmailHash(key: string, value: string): Attr { return HashString(key, value); }
export function IpHash(key: string, value: string): Attr { return HashString(key, value); }

// Additional canonical field
export function RegionEx(region: string): Attr { return String('region', region); }

// Checkout domain pack
export function CheckoutCartItemCount(count: number): Attr { return Int('checkout.cart_item_count', count); }
export function CheckoutCartTotal(totalCents: number): Attr { return Int64('checkout.cart_total_cents', totalCents); }
export function CheckoutPaymentMethod(method: string): Attr { return String('checkout.payment_method', method); }
export function CheckoutStatus(status: string): Attr { return String('checkout.status', status); }

// Payment domain pack
export function PaymentProvider(provider: string): Attr { return String('payment.provider', provider); }
export function PaymentMethod(method: string): Attr { return String('payment.method', method); }
export function PaymentIntentId(id: string): Attr { return String('payment.intent_id', id); }
export function PaymentFailureCode(code: string): Attr { return String('payment.failure_code', code); }
export function PaymentRetryAttempt(attempt: number): Attr { return Int('payment.retry_attempt', attempt); }

// Billing domain pack
export function BillingPlan(plan: string): Attr { return String('billing.plan', plan); }
export function BillingSubscriptionId(id: string): Attr { return String('billing.subscription_id', id); }
export function BillingInvoiceId(id: string): Attr { return String('billing.invoice_id', id); }
export function BillingAmount(cents: number): Attr { return Int64('billing.amount_cents', cents); }
export function BillingInterval(interval: string): Attr { return String('billing.interval', interval); }

// Agent/AI domain pack
export function AgentName(name: string): Attr { return String('agent.name', name); }
export function AgentProvider(provider: string): Attr { return String('agent.provider', provider); }
export function AgentModel(model: string): Attr { return String('agent.model', model); }
export function AgentRunType(runType: string): Attr { return String('agent.run_type', runType); }
export function AgentToolName(name: string): Attr { return String('agent.tool_name', name); }
export function AgentToolOutcome(outcome: string): Attr { return String('agent.tool_outcome', outcome); }
export function AgentInputTokens(count: number): Attr { return Int64('agent.input_tokens', count); }
export function AgentOutputTokens(count: number): Attr { return Int64('agent.output_tokens', count); }
export function AgentCost(cents: number): Attr { return Int64('agent.cost_micros', cents); }

// RAG domain pack
export function RagIndex(index: string): Attr { return String('rag.index', index); }
export function RagEmbeddingModel(model: string): Attr { return String('rag.embedding_model', model); }
export function RagChunksRetrieved(count: number): Attr { return Int('rag.chunks_retrieved', count); }
export function RagTopScore(score: number): Attr { return Float64('rag.top_score', score); }
export function RagQueryHash(hash: string): Attr { return String('rag.query_hash', hash); }
export function RagCitationCount(count: number): Attr { return Int('rag.citation_count', count); }
export function RagRetrievalLatency(ms: number): Attr { return Float64('rag.retrieval_latency_ms', ms); }

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

// camelCase aliases for new domain helpers
export const paymentId = PaymentID;
export const subscriptionId = SubscriptionID;
export const invoiceId = InvoiceID;
export const jobId = JobID;
export const messageId = MessageID;
export const correlationId = CorrelationID;
export const commitSha = CommitSha;
export const release = Release;
export const money = Money;
export const percent = Percent;
export const bytes = Bytes;
export const httpStatus = HttpStatus;
export const statusCodeFn = StatusCode;
export const errorCodeExt = ErrorCodeExt;
export const bucket = Bucket;
export const tags = Tags;
export const masked = Masked;
export const url = Url;
export const emailHash = EmailHash;
export const ipHash = IpHash;
export const regionEx = RegionEx;
export const checkoutCartItemCount = CheckoutCartItemCount;
export const checkoutCartTotal = CheckoutCartTotal;
export const checkoutPaymentMethod = CheckoutPaymentMethod;
export const checkoutStatus = CheckoutStatus;
export const paymentProvider = PaymentProvider;
export const paymentMethod = PaymentMethod;
export const paymentIntentId = PaymentIntentId;
export const paymentFailureCode = PaymentFailureCode;
export const paymentRetryAttempt = PaymentRetryAttempt;
export const billingPlan = BillingPlan;
export const billingSubscriptionId = BillingSubscriptionId;
export const billingInvoiceId = BillingInvoiceId;
export const billingAmount = BillingAmount;
export const billingInterval = BillingInterval;
export const agentName = AgentName;
export const agentProvider = AgentProvider;
export const agentModel = AgentModel;
export const agentRunType = AgentRunType;
export const agentToolName = AgentToolName;
export const agentToolOutcome = AgentToolOutcome;
export const agentInputTokens = AgentInputTokens;
export const agentOutputTokens = AgentOutputTokens;
export const agentCost = AgentCost;
export const ragIndex = RagIndex;
export const ragEmbeddingModel = RagEmbeddingModel;
export const ragChunksRetrieved = RagChunksRetrieved;
export const ragTopScore = RagTopScore;
export const ragQueryHash = RagQueryHash;
export const ragCitationCount = RagCitationCount;
export const ragRetrievalLatency = RagRetrievalLatency;

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
    // Use property descriptors to preserve getters/setters instead of Object.assign
    const descriptors = Object.getOwnPropertyDescriptors(this);
    Object.defineProperties(ev, descriptors);
    // Deep-clone mutable collections so the clone is independent
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
