export const ErrorCategory = {
  InvalidConfiguration: 'invalid_configuration',
  Transport: 'transport',
  Authentication: 'authentication',
  Scope: 'scope',
  Diagnostics: 'diagnostics',
  CompilerUnavailable: 'compiler_unavailable',
  Execution: 'execution',
  Timeout: 'timeout',
  MalformedResponse: 'malformed_response',
} as const;

export type ErrorCategoryName = typeof ErrorCategory[keyof typeof ErrorCategory];

export interface QueryValue {
  type: string;
  value: unknown;
}

export interface QueryColumn {
  name: string;
  type?: string;
  nullable?: boolean;
}

export interface QueryResult {
  columns: QueryColumn[];
  rows: Record<string, unknown>[];
  durationMs: number;
  rowCount: number;
}

export class QueryError extends Error {
  readonly category: ErrorCategoryName;
  readonly status?: number;
  readonly diagnostics: Record<string, unknown>[];

  constructor(message: string, category: ErrorCategoryName, status?: number, diagnostics: Record<string, unknown>[] = []) {
    super(message);
    this.name = 'QueryError';
    this.category = category;
    this.status = status;
    this.diagnostics = diagnostics;
  }
}

export interface ConnectionConfig {
  dsn?: string;
  endpoint?: string;
  collector?: string;
  apiKey?: string;
  username?: string;
  password?: string;
  env?: string;
  service?: string;
  timeoutMs?: number;
  maxResponseBytes?: number;
  fetch?: typeof globalThis.fetch;
}

interface ResolvedConfig {
  endpoint: string;
  collector: string;
  apiKey: string;
  username: string;
  password: string;
  env: string;
  service: string;
  timeoutMs: number;
  maxResponseBytes: number;
  fetch: typeof globalThis.fetch;
}

export class Client {
  private readonly config: ResolvedConfig;

  constructor(config: ConnectionConfig = {}) {
    const resolved = resolveConfig(config);
    this.config = resolved;
  }

  async query(source: string, parameters: Record<string, QueryValue | unknown> = {}, limit = 1000): Promise<QueryResult> {
    if (!source.trim()) {
      throw new QueryError('LQL query source is required', ErrorCategory.InvalidConfiguration);
    }
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), this.config.timeoutMs);
    const normalizedLimit = Math.max(1, Math.min(Math.trunc(limit), 1000));
    const encodedParameters: Record<string, QueryValue> = {};
    for (const [name, value] of Object.entries(parameters)) {
      encodedParameters[name] = isQueryValue(value)
        ? value
        : { type: inferType(value), value };
    }
    const endpoint = `${this.config.endpoint}/collectors/${encodeURIComponent(this.config.collector)}/lql/query`;
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (this.config.apiKey) {
      headers.Authorization = `Bearer ${this.config.apiKey}`;
    } else if (this.config.username) {
      headers.Authorization = `Basic ${base64(`${this.config.username}:${this.config.password}`)}`;
    }
    if (this.config.env) headers['X-Loza-Env'] = this.config.env;
    if (this.config.service) headers['X-Loza-Service'] = this.config.service;
    try {
      const response = await this.config.fetch(endpoint, {
        method: 'POST',
        headers,
        body: JSON.stringify({ query: source, parameters: encodedParameters, limit: normalizedLimit }),
        signal: controller.signal,
      });
      const raw = await response.arrayBuffer();
      if (raw.byteLength > this.config.maxResponseBytes) {
        throw new QueryError('LQL response exceeds the configured size limit', ErrorCategory.MalformedResponse);
      }
      const text = new TextDecoder().decode(raw);
      if (!response.ok) {
        throw decodeHTTPError(response.status, text);
      }
      return decodeResult(text);
    } catch (error) {
      if (error instanceof QueryError) throw error;
      if (controller.signal.aborted) {
        throw new QueryError('LQL query timed out', ErrorCategory.Timeout);
      }
      throw new QueryError('LQL query transport failed', ErrorCategory.Transport);
    } finally {
      clearTimeout(timeout);
    }
  }
}

function resolveConfig(input: ConnectionConfig): ResolvedConfig {
  const dsn = input.dsn || readEnv('LOZA_DSN');
  const parsed = dsn ? parseDSN(dsn) : undefined;
  const endpoint = input.endpoint || parsed?.endpoint || '';
  const collector = input.collector || parsed?.collector || '';
  const username = input.username || parsed?.username || readEnv('LOZA_USERNAME');
  const password = input.password || parsed?.password || readEnv('LOZA_PASSWORD');
  const apiKey = input.apiKey || readEnv('LOZA_API_KEY');
  const env = input.env || parsed?.env || '';
  const service = input.service || parsed?.service || '';
  let endpointURL: URL;
  try {
    endpointURL = new URL(endpoint);
  } catch {
    throw new QueryError('invalid LQL connection configuration: endpoint is required', ErrorCategory.InvalidConfiguration);
  }
  if (!['http:', 'https:'].includes(endpointURL.protocol) || endpointURL.username || endpointURL.password) {
    throw new QueryError('invalid LQL connection configuration: endpoint must be HTTP(S) without userinfo', ErrorCategory.InvalidConfiguration);
  }
  if (!/^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(collector)) {
    throw new QueryError('invalid LQL connection configuration: collector slug is required', ErrorCategory.InvalidConfiguration);
  }
  if (username && !password && !username.startsWith('lx_pub_')) {
    throw new QueryError('invalid LQL connection configuration: basic username requires a password', ErrorCategory.InvalidConfiguration);
  }
  if (username && !apiKey && endpointURL.protocol === 'http:' && !isLocalhost(endpointURL.hostname)) {
    throw new QueryError('invalid LQL connection configuration: basic authentication requires TLS', ErrorCategory.InvalidConfiguration);
  }
  return {
    endpoint: endpoint.replace(/\/$/, ''), collector, apiKey, username, password, env, service,
    timeoutMs: input.timeoutMs && input.timeoutMs > 0 ? input.timeoutMs : 30_000,
    maxResponseBytes: input.maxResponseBytes && input.maxResponseBytes > 0 ? input.maxResponseBytes : 8 * 1024 * 1024,
    fetch: input.fetch || globalThis.fetch,
  };
}

function parseDSN(raw: string): { endpoint: string; collector: string; username: string; password: string; env: string; service: string } {
  let parsed: URL;
  try {
    parsed = new URL(raw.replace(/^loza:\/\//, 'http://'));
  } catch {
    throw new QueryError('invalid LQL connection configuration: invalid DSN', ErrorCategory.InvalidConfiguration);
  }
  const collector = decodeURIComponent(parsed.pathname.replace(/^\//, ''));
  if (!collector || !parsed.hostname) throw new QueryError('invalid LQL connection configuration: invalid DSN', ErrorCategory.InvalidConfiguration);
  const tlsValue = parsed.searchParams.get('tls');
  const tls = tlsValue === 'true' || (tlsValue !== 'false' && !isLocalhost(parsed.hostname));
  if (tlsValue && !['true', 'false', 'auto'].includes(tlsValue)) throw new QueryError('invalid LQL connection configuration: invalid DSN', ErrorCategory.InvalidConfiguration);
  const port = parsed.port || (isLocalhost(parsed.hostname) ? '9308' : tls ? '443' : '80');
  const username = decodeURIComponent(parsed.username);
  const password = decodeURIComponent(parsed.password);
  if (username && !password && !username.startsWith('lx_pub_')) throw new QueryError('invalid LQL connection configuration: invalid DSN credentials', ErrorCategory.InvalidConfiguration);
  return {
    endpoint: `${tls ? 'https' : 'http'}://${parsed.hostname}:${port}`,
    collector, username, password,
    env: parsed.searchParams.get('env') || 'default', service: parsed.searchParams.get('service') || '',
  };
}

function decodeResult(text: string): QueryResult {
  let payload: unknown;
  try { payload = JSON.parse(text); } catch { throw new QueryError('LQL response has an invalid result envelope', ErrorCategory.MalformedResponse); }
  if (!isRecord(payload) || !Array.isArray(payload.columns) || !Array.isArray(payload.rows)) {
    throw new QueryError('LQL response has an invalid result envelope', ErrorCategory.MalformedResponse);
  }
  const columns: QueryColumn[] = [];
  for (const column of payload.columns) {
    if (typeof column === 'string') columns.push({ name: column });
    else if (isRecord(column) && typeof column.name === 'string') columns.push({ name: column.name, type: typeof column.type === 'string' ? column.type : undefined, nullable: typeof column.nullable === 'boolean' ? column.nullable : undefined });
    else throw new QueryError('LQL response has an invalid result envelope', ErrorCategory.MalformedResponse);
  }
  if (!payload.rows.every(isRecord)) throw new QueryError('LQL response has an invalid result envelope', ErrorCategory.MalformedResponse);
  return { columns, rows: payload.rows, durationMs: typeof payload.duration_ms === 'number' ? payload.duration_ms : 0, rowCount: typeof payload.row_count === 'number' ? payload.row_count : payload.rows.length };
}

function decodeHTTPError(status: number, text: string): QueryError {
  let payload: unknown = {};
  try { payload = JSON.parse(text); } catch { /* use stable fallback */ }
  const message = isRecord(payload) && (typeof payload.error === 'string' ? payload.error : typeof payload.message === 'string' ? payload.message : '') || `LQL query failed with HTTP ${status}`;
  const diagnostics = isRecord(payload) && Array.isArray(payload.diagnostics) && payload.diagnostics.every(isRecord) ? payload.diagnostics : [];
  const category = status === 400 ? ErrorCategory.Diagnostics : status === 401 ? ErrorCategory.Authentication : status === 403 ? ErrorCategory.Scope : status === 503 ? ErrorCategory.CompilerUnavailable : ErrorCategory.Execution;
  return new QueryError(message, category, status, diagnostics);
}

function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value); }
function isQueryValue(value: unknown): value is QueryValue { return isRecord(value) && typeof value.type === 'string' && 'value' in value; }
function inferType(value: unknown): string { return value === null ? 'null' : typeof value === 'boolean' ? 'bool' : typeof value === 'number' ? (Number.isInteger(value) ? 'int' : 'float') : 'string'; }
function isLocalhost(host: string): boolean { return host === 'localhost' || host === '127.0.0.1' || host === '::1'; }
function readEnv(name: string): string { return typeof process !== 'undefined' ? process.env[name] || '' : ''; }
function base64(value: string): string {
  if (typeof btoa !== 'function') {
    throw new QueryError('LQL basic authentication is unavailable in this runtime', ErrorCategory.InvalidConfiguration);
  }
  return btoa(value);
}
