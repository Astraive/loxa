/**
 * Parses loza:// connection URIs into resolved HTTP/HTTPS/WebSocket endpoints.
 *
 * The loza:// URI is the standard connection string for Loza Collector.
 * It resolves to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints — it is NOT a wire protocol.
 *
 * Format:
 *   loza://[username:password@][host][:port]/[collector]?env=<env>&service=<service>&tls=<true|false|auto>&transport=<http|otlp|grpc>
 *
 * Private credentials use username:password. Public lx_pub_... credentials use
 * an explicitly empty password (lx_pub_...:) as a bearer capability.

/** Parsed and resolved values from a loza:// connection URI. */
export interface LozaDSN {
  scheme: string;         // always "loza"
  username?: string;      // decoded private key ID or public bearer capability
  password?: string;      // decoded private key secret; empty for public capabilities
  host: string;           // hostname (no port)
  port: number;           // resolved port number
  collectorName: string;  // canonical collector slug from the required path
  project: string;        // compatibility alias for collectorName
  env: string;            // environment name (default: "default")
  service: string;        // optional service name
  tls: boolean;           // whether to use HTTPS
  transport: string;      // "http", "otlp", or "grpc" (default: "http")
  baseURL: string;        // resolved http(s)://host:port (never includes userinfo)
  eventsURL: string;      // base + /collectors/{collector}/events
  batchURL: string;       // base + /collectors/{collector}/events/batch
  otlpURL: string;        // base + /collectors/{collector}/otlp/logs
  tailWSURL: string;      // ws(s)://host:port/collectors/{collector}/tail
  lqlURL: string;         // base + /collectors/{collector}/lql/query
  lqlQueryURL: string;    // compatibility alias for lqlURL
  toString(): string;
  toJSON(): Omit<LozaDSN, 'username' | 'password' | 'toString' | 'toJSON'>;
}

const USERINFO_RESERVED = /[:\s]/;
const PASSWORD_RESERVED = /[/:?#\[\]@!$&'()*+,;=]/;

function isLocalhost(host: string): boolean {
  return host === 'localhost' || host === '127.0.0.1' || host === '::1';
}

export function isPublicDSNUsername(username: string): boolean {
  const prefix = 'lx_pub_';
  return username.startsWith(prefix) && username.length > prefix.length;
}

/**
 * Parse a raw loza:// connection URI into a LozaDSN.
 *
 * Validation rules:
 *   - Collector path is required (loza://host is rejected)
 *   - Private userinfo must contain non-empty username/password.
 *   - Public userinfo is lx_pub_...: with an explicitly empty password.
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
    if (!rawUsername) {
      throw new Error('invalid Loza DSN: credentials require username:password or lx_pub_...:');
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
    if (!username || (password === '' && !isPublicDSNUsername(username))) {
      throw new Error('invalid Loza DSN: credentials require username:password or lx_pub_...:');
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

  // The required path is the canonical collector identity. project remains
  // available for compatibility with existing SDK consumers.
  const collectorName = url.pathname.replace(/^\//, '');
  if (!collectorName) {
    throw new Error('invalid Loza DSN: collector path is required, e.g. loza://host/my-collector');
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

  const collectorPath = encodeURIComponent(collectorName);
  const collectorBaseURL = `${baseURL}/collectors/${collectorPath}`;
  const collectorTailBaseURL = `${wsScheme}://${hostPart}:${port}/collectors/${collectorPath}`;
  const redacted = {
    scheme: 'loza',
    host,
    port,
    collectorName,
    project: collectorName,
    env,
    service,
    tls,
    transport,
    baseURL,
    eventsURL: `${collectorBaseURL}/events`,
    batchURL: `${collectorBaseURL}/events/batch`,
    otlpURL: `${collectorBaseURL}/otlp/logs`,
    tailWSURL: `${collectorTailBaseURL}/tail`,
    lqlURL: `${collectorBaseURL}/lql/query`,
    lqlQueryURL: `${collectorBaseURL}/lql/query`,
  };

  return {
    ...redacted,
    ...(username !== undefined ? { username, password } : {}),
    toString: () => baseURL,
    toJSON: () => redacted,
  };
}
