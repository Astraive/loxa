import type { Sink } from './sink.ts';
import fs from 'node:fs';
import http from 'node:http';
import https from 'node:https';
import { gzip as zlibGzip } from 'node:zlib';
import { promisify } from 'node:util';
import { parseCollectorResponse } from '../generated/spec-contract.ts';
import type { CollectorResponse } from '../generated/spec-contract.ts';

const gzipAsync = promisify(zlibGzip);

/** Stats handler for collector acknowledgements. */
export interface StatsHandler {
  onCollectorAck?(data: {
    acks: CollectorResponse['acks'];
    errors: CollectorResponse['errors'];
    requestId: string;
    deduped: number;
  }): void;
}

export interface DeliveryFailureHandler extends StatsHandler {
  onDeliveryFailed?(event: unknown, error: unknown): void;
}

/** Stdout sink — writes NDJSON to process.stdout. */
export class StdoutSink implements Sink {
  name() { return 'stdout'; }
  write(encoded: string): void {
    process.stdout.write(encoded + '\n');
  }
  flush() {}
  close() {}
}

/** Stderr sink — writes NDJSON to process.stderr. */
export class StderrSink implements Sink {
  name() { return 'stderr'; }
  write(encoded: string): void {
    process.stderr.write(encoded + '\n');
  }
  flush() {}
  close() {}
}

export class FileSink implements Sink {
  private path: string;
  constructor(path: string) {
    this.path = path;
  }
  name() { return 'file'; }
  write(encoded: string): void { fs.appendFileSync(this.path, encoded + '\n'); }
  flush() {}
  close() {}
}

export class RotatingFileSink extends FileSink {}

/** Noop sink — discards all events. */
export class NoopSink implements Sink {
  name() { return 'noop'; }
  write(_encoded: string): void {}
  flush() {}
  close() {}
}

/** Memory sink — captures events in memory for testing. */
export class MemorySink implements Sink {
  private events: string[] = [];

  name() { return 'memory'; }
  write(encoded: string): void {
    this.events.push(encoded);
  }
  flush() {}
  close() {}

  getEvents(): string[] { return [...this.events]; }
  getLength(): number { return this.events.length; }
  clear(): void { this.events = []; }
}

/** HTTP batch sink — sends events to loxa-collector via HTTP. */
export class HTTPBatchSink implements Sink {
  private endpoint: string;
  private apiKey: string;
  private authHeader: string;
  private sdkName: string;
  private sdkVersion: string;
  private service: string;
  private timeout: number;
  private retries: number;
  private baseDelay: number;
  private maxDelay: number;
  private enableCompression: boolean;
  private ndjson: boolean;
  private buffer: string[] = [];
  private flushTimer: ReturnType<typeof setTimeout> | null = null;
  private batchSize: number;
  private flushIntervalMs: number;
  private statsHandler: StatsHandler | null;
  private _lastResponse: CollectorResponse | null = null;

  constructor(opts: {
    endpoint: string;
    apiKey?: string;
    authHeader?: string;
    sdkName?: string;
    sdkVersion?: string;
    service?: string;
    timeout?: number;
    retries?: number;
    baseDelay?: number;
    maxDelay?: number;
    enableCompression?: boolean;
    ndjson?: boolean;
    batchSize?: number;
    flushIntervalMs?: number;
    statsHandler?: StatsHandler;
  }) {
    this.endpoint = opts.endpoint;
    this.apiKey = opts.apiKey || '';
    this.authHeader = opts.authHeader || 'x-loxa-api-key';
    this.sdkName = opts.sdkName || 'loxa-js';
    this.sdkVersion = opts.sdkVersion || '1.0.0';
    this.service = opts.service || '';
    this.timeout = opts.timeout || 2000;
    this.retries = opts.retries ?? 3;
    this.baseDelay = opts.baseDelay ?? 100;
    this.maxDelay = opts.maxDelay ?? 30000;
    this.enableCompression = opts.enableCompression ?? true;
    this.ndjson = opts.ndjson ?? false;
    this.batchSize = opts.batchSize || 50;
    this.flushIntervalMs = opts.flushIntervalMs || 5000;
    this.statsHandler = opts.statsHandler || null;
  }

  name() { return 'httpbatch'; }

  /** Get the last parsed collector response. */
  get lastCollectorResponse(): CollectorResponse | null { return this._lastResponse; }

  write(encoded: string): void {
    this.buffer.push(encoded);
    if (this.buffer.length >= this.batchSize) {
      this.flush();
    } else if (!this.flushTimer) {
      this.flushTimer = setTimeout(() => this.flush(), this.flushIntervalMs);
    }
  }

  async flush(): Promise<void> {
    if (this.flushTimer) {
      clearTimeout(this.flushTimer);
      this.flushTimer = null;
    }
    if (this.buffer.length === 0) return;

    const events = [...this.buffer];
    this.buffer = [];

    const envelope = this.ndjson
      ? events.join('\n')
      : JSON.stringify({
          api_version: 'v1',
          source: { sdk: this.sdkName, version: this.sdkVersion, service: this.service },
          events: events.map(e => JSON.parse(e)),
        });

    const body = this.enableCompression
      ? await gzipAsync(Buffer.from(envelope, 'utf-8'))
      : Buffer.from(envelope, 'utf-8');

    let lastError: Error | null = null;
    for (let attempt = 0; attempt <= this.retries; attempt++) {
      try {
        const result = await this.post(body);
        this._lastResponse = result.response;
        this.notifyCollectorAck(result.response);

        const outcome = this.classifyOutcome(result.statusCode, result.response);
        if (outcome === 'success') return;

        const retryAfterMs = result.response.retry_after_ms
          ?? this.parseRetryAfter(result.retryAfterHeader);

        lastError = new Error(
          `collector reported ${outcome === 'retryable' ? 'retryable errors' : 'batch failure'}: ` +
          `${result.response.error || result.response.reason || `accepted=${result.response.accepted} rejected=${result.response.rejected}`}`
        );

        if (outcome === 'retryable' && attempt < this.retries) {
          const delay = retryAfterMs
            ?? Math.min(this.baseDelay * 2 ** attempt, this.maxDelay);
          await sleep(delay);
          continue;
        }
        throw lastError;
      } catch (err) {
        if (err instanceof Error && err.message.startsWith('collector reported')) throw err;
        lastError = err instanceof Error ? err : new Error(String(err));
        if (attempt < this.retries) {
          await sleep(Math.min(this.baseDelay * 2 ** attempt, this.maxDelay));
          continue;
        }
        throw new Error(`collector send failed: ${lastError.message}`);
      }
    }
    if (lastError) throw new Error(`collector send failed: ${lastError.message}`);
  }

  close(): void {
    if (this.flushTimer) {
      clearTimeout(this.flushTimer);
      this.flushTimer = null;
    }
  }

  private classifyOutcome(statusCode: number, response: CollectorResponse): 'success' | 'retryable' | 'permanent' {
    // Check response-level retryable errors
    if (response.acks?.some(a => a.retryable)) return 'retryable';
    if (response.errors?.some(e => e.retryable)) return 'retryable';

    // HTTP status-based classification
    if (statusCode === 429 || statusCode === 503) return 'retryable';
    if (statusCode >= 300) return 'permanent';

    // Response status-based
    if (response.status === 'rejected' || response.status === 'invalid') {
      return response.accepted > 0 ? 'retryable' : 'permanent';
    }

    return 'success';
  }

  private parseRetryAfter(header: string | null | undefined): number | null {
    if (!header) return null;
    const trimmed = header.trim();
    if (!trimmed) return null;

    // Try numeric seconds
    const num = Number(trimmed);
    if (!isNaN(num)) return num * 1000;

    // Try HTTP-date
    try {
      const date = new Date(trimmed);
      if (!isNaN(date.getTime())) {
        const ms = date.getTime() - Date.now();
        return ms > 0 ? ms : 0;
      }
    } catch { /* ignore */ }

    return null;
  }

  private notifyCollectorAck(response: CollectorResponse): void {
    if (!this.statsHandler?.onCollectorAck) return;
    try {
      this.statsHandler.onCollectorAck({
        acks: response.acks || [],
        errors: response.errors || [],
        requestId: response.request_id || '',
        deduped: response.deduped || 0,
      });
    } catch { /* swallow */ }
  }

  private post(body: Buffer): Promise<{ statusCode: number; retryAfterHeader: string | null; response: CollectorResponse }> {
    return new Promise((resolve, reject) => {
      const url = new URL(this.endpoint);
      const isHttps = url.protocol === 'https:';
      const mod = isHttps ? https : http;

      const headers: Record<string, string> = {
        'Content-Type': this.ndjson ? 'application/x-ndjson' : 'application/json',
        'Content-Length': body.length.toString(),
      };
      if (this.apiKey) headers[this.authHeader] = this.apiKey;
      if (this.enableCompression) headers['Content-Encoding'] = 'gzip';

      const req = mod.request({
        hostname: url.hostname,
        port: url.port || (isHttps ? 443 : 80),
        path: url.pathname,
        method: 'POST',
        headers,
        timeout: this.timeout,
      }, (res) => {
        let data = '';
        res.on('data', (chunk: Buffer) => { data += chunk.toString(); });
        res.on('end', () => {
          const statusCode = res.statusCode || 0;
          const retryAfterHeader = res.headers['retry-after'] || null;

          let response: CollectorResponse;
          try {
            response = parseCollectorResponse(data);
          } catch {
            // If response isn't valid JSON, create a minimal response
            response = {
              request_id: '',
              status: statusCode >= 200 && statusCode < 300 ? 'accepted' : 'rejected',
              accepted: statusCode >= 200 && statusCode < 300 ? 1 : 0,
              rejected: statusCode >= 200 && statusCode < 300 ? 0 : 1,
              invalid: 0,
              acks: [],
            };
          }

          resolve({ statusCode, retryAfterHeader, response });
        });
      });

      req.on('error', reject);
      req.on('timeout', () => { req.destroy(); reject(new Error('timeout')); });
      req.write(body);
      req.end();
    });
  }
}

export class CollectorSink extends HTTPBatchSink {}

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

/** Options for HTTPBatchSink constructor. */
export interface HTTPBatchSinkOptions {
  endpoint: string;
  apiKey?: string;
  authHeader?: string;
  sdkName?: string;
  sdkVersion?: string;
  service?: string;
  timeout?: number;
  retries?: number;
  baseDelay?: number;
  maxDelay?: number;
  enableCompression?: boolean;
  ndjson?: boolean;
  batchSize?: number;
  flushIntervalMs?: number;
  statsHandler?: StatsHandler;
}

// --- Lowercase factory functions ---

export function stdoutSink(): StdoutSink { return new StdoutSink(); }
export function stderrSink(): StderrSink { return new StderrSink(); }
export function noopSink(): NoopSink { return new NoopSink(); }
export function memorySink(): MemorySink { return new MemorySink(); }
export function fileSink(path: string): FileSink { return new FileSink(path); }
export function rotatingFileSink(path: string): RotatingFileSink { return new RotatingFileSink(path); }
export function collectorSink(opts: HTTPBatchSinkOptions): CollectorSink { return new CollectorSink(opts); }
export function httpBatchSink(opts: HTTPBatchSinkOptions): HTTPBatchSink { return new HTTPBatchSink(opts); }
