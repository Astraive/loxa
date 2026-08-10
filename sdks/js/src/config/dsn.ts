/**
 * Parses loza:// connection URIs into resolved HTTP/HTTPS/WebSocket endpoints.
 *
 * The loza:// URI is the standard connection string for Loza Collector.
 * It resolves to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints — it is NOT a wire protocol.
 *
 * Format:
 *   loza://[host][:port]/[project]?env=<env>&service=<service>&tls=<true|false|auto>&transport=<http|otlp|grpc>
 */

/** Parsed and resolved values from a loza:// connection URI. */
export interface LozaDSN {
  scheme: string;    // always "loza"
  host: string;      // hostname (no port)
  port: number;      // resolved port number
  project: string;   // path segment (project name)
  env: string;       // environment name (default: "default")
  service: string;   // optional service name
  tls: boolean;      // whether to use HTTPS
  transport: string; // "http", "otlp", or "grpc" (default: "http")
  baseURL: string;   // resolved http(s)://host:port
  eventsURL: string; // base + /events
  batchURL: string;  // base + /events/batch
  otlpURL: string;   // base + /otlp/logs
  tailWSURL: string; // ws(s)://host:port/tail
}

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
 *   - No userinfo allowed (loza://user:pass@host/project is rejected)
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

  // Reject userinfo (API keys must not be in the URL).
  if (url.username || url.password) {
    throw new Error('invalid Loza DSN: do not put API keys in the URL, use LOZA_API_KEY instead');
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

  return {
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
}
