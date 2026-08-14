/**
 * Parses loza:// connection URIs into resolved HTTP/HTTPS/WebSocket endpoints.
 *
 * The loza:// URI is the standard connection string for Loza Collector.
 * It resolves to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints — it is NOT a wire protocol.
 *
 * Format:
 *   loza://[username:password@][host][:port]/[project]?env=<env>&service=<service>&tls=<true|false|auto>&transport=<http|otlp|grpc>
 */

/** Parsed and resolved values from a loza:// connection URI. */
export interface LozaDSN {
  scheme: string;    // always "loza"
  username?: string; // decoded Collector key ID
  password?: string; // decoded Collector key secret
  host: string;      // hostname (no port)
  port: number;      // resolved port number
  project: string;   // path segment (project name)
  env: string;       // environment name (default: "default")
  service: string;   // optional service name
  tls: boolean;      // whether to use HTTPS
  transport: string; // "http", "otlp", or "grpc" (default: "http")
  baseURL: string;   // resolved http(s)://host:port (never includes userinfo)
  eventsURL: string; // base + /events
  batchURL: string;  // base + /events/batch
  otlpURL: string;   // base + /otlp/logs
  tailWSURL: string; // ws(s)://host:port/tail
  toString(): string;
  toJSON(): Omit<LozaDSN, 'username' | 'password' | 'toString' | 'toJSON'>;
}

const USERINFO_RESERVED = /[:\s]/;
const PASSWORD_RESERVED = /[/:?#\[\]@!$&'()*+,;=]/;

function isLocalhost(host: string): boolean {
  return host === 'localhost' || host === '127.0.0.1' || host === '::1';
}

/**
 * Parse a raw loza:// connection URI into a LozaDSN.
 *
 * Validation rules:
 *   - Scheme must be loza://
 *   - Host is required (loza:// or loza:///project are rejected)
 *   - Project path is required (loza://host is rejected)
 *   - Userinfo, when present, must contain non-empty username/password.
 *   - Userinfo is percent-decoded and malformed escapes are rejected.
 *   - Basic usernames cannot contain ':' or whitespace.
 *   - Password reserved characters must be percent-encoded.
 *   - tls must be "true", "false", or "auto"
 *   - transport must be "http", "otlp", or "grpc"
 *   - Port must be 1-65535 if specified
 *
 * TLS defaults:
 *   - localhost/127.0.0.1/::1 -> false
 *   - everything else -> true
 *
 * Port defaults:
 *   - tls=true -> 443
 *   - tls=false -> 80
 *   - localhost without explicit port -> 9308
 */
export function parse(raw: string): LozaDSN {
  if (raw === '') {
    throw new Error('invalid Loza DSN: empty string');
  }

  if (!raw.startsWith('loza://')) {
    throw new Error('invalid Loza DSN: scheme must be loza://');
  }

  let url: URL;
  try {
    url = new URL(raw);
  } catch (e) {
    throw new Error(`invalid Loza DSN: ${e instanceof Error ? e.message : String(e)}`);
  }

  // Parse userinfo from the raw authority before URL normalisation. URL.username
  // remains percent-encoded in Node, and inspecting the raw value lets us reject
  // unescaped password delimiters without ever retaining credentials in output.
  const authority = raw.slice('loza://'.length).split(/[/?#]/, 1)[0] || '';
  const at = authority.lastIndexOf('@');
  let username: string | undefined;
  let password: string | undefined;
  if (at >= 0) {
    const userinfo = authority.slice(0, at);
    const separator = userinfo.indexOf(':');
    if (separator < 0 || separator !== userinfo.lastIndexOf(':')) {
      throw new Error('invalid Loza DSN: userinfo must be username:password');
    }
    const rawUsername = userinfo.slice(0, separator);
    const rawPassword = userinfo.slice(separator + 1);
    if (!rawUsername || !rawPassword) {
      throw new Error('invalid Loza DSN: username and password are required');
    }
    if (PASSWORD_RESERVED.test(rawPassword)) {
      throw new Error('invalid Loza DSN: reserved password characters must be percent-encoded');
    }
    try {
      username = decodeURIComponent(rawUsername);
      password = decodeURIComponent(rawPassword);
    } catch {
      throw new Error('invalid Loza DSN: malformed percent-encoded userinfo');
    }
    if (!username || !password) {
      throw new Error('invalid Loza DSN: username and password are required');
    }
    if (USERINFO_RESERVED.test(username)) {
      throw new Error('invalid Loza DSN: username must not contain ":" or whitespace');
    }
  }

  // Reject userinfo if URL parsing found an authority shape we did not parse.
  // (For example, URL may accept an empty userinfo as `url.username/password`.)
  if (at < 0 && (url.username || url.password)) {
    throw new Error('invalid Loza DSN: malformed userinfo');
  }

  // Strip brackets from IPv6 hostname (URL API may include them for non-standard schemes).
  const host = url.hostname.replace(/^\[|\]$/g, '');
  if (!host) {
    throw new Error('invalid Loza DSN: host is required');
  }

  // Project is the path segment without leading slash.
  const project = url.pathname.replace(/^\//, '');
  if (!project) {
    throw new Error('invalid Loza DSN: project path is required, e.g. loza://host/my-project');
  }

  // ── TLS default ──────────────────────────────────────────────────────────
  let tls = !isLocalhost(host);
  const tlsParam = url.searchParams.get('tls');
  if (tlsParam !== null) {
    switch (tlsParam) {
      case 'true':
        tls = true;
        break;
      case 'false':
        tls = false;
        break;
      case 'auto':
        // keep the computed default
        break;
      default:
        throw new Error(`invalid Loza DSN: tls must be true, false, or auto, got "${tlsParam}"`);
    }
  }

  // ── Port default ─────────────────────────────────────────────────────────
  let port = tls ? 443 : 80;
  if (isLocalhost(host) && url.port === '') {
    port = 9308;
  }
  if (url.port !== '') {
    const p = parseInt(url.port, 10);
    if (isNaN(p) || p < 1 || p > 65535) {
      throw new Error(`invalid Loza DSN: invalid port "${url.port}"`);
    }
    port = p;
  }

  // ── Transport ────────────────────────────────────────────────────────────
  let transport = 'http';
  const transportParam = url.searchParams.get('transport');
  if (transportParam !== null) {
    switch (transportParam) {
      case 'http':
      case 'otlp':
      case 'grpc':
        transport = transportParam;
        break;
      default:
        throw new Error(`invalid Loza DSN: transport must be http, otlp, or grpc, got "${transportParam}"`);
    }
  }

  // ── Env ──────────────────────────────────────────────────────────────────
  const env = url.searchParams.get('env') || 'default';
  const service = url.searchParams.get('service') || '';

  // ── Build resolved URLs ──────────────────────────────────────────────────
  const scheme = tls ? 'https' : 'http';
  const wsScheme = tls ? 'wss' : 'ws';

  // IPv6 addresses must be bracketed in URLs per RFC 2732/3986.
  const hostPart = host.includes(':') ? `[${host}]` : host;
  const baseURL = `${scheme}://${hostPart}:${port}`;

  const redacted = {
    scheme: 'loza',
    host,
    port,
    project,
    env,
    service,
    tls,
    transport,
    baseURL,
    eventsURL: `${baseURL}/events`,
    batchURL: `${baseURL}/events/batch`,
    otlpURL: `${baseURL}/otlp/logs`,
    tailWSURL: `${wsScheme}://${hostPart}:${port}/tail`,
  };

  return {
    ...redacted,
    ...(username !== undefined ? { username, password } : {}),
    toString: () => baseURL,
    toJSON: () => redacted,
  };
}
