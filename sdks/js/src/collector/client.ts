import http from 'node:http';
import https from 'node:https';
import { gzip as zlibGzip } from 'node:zlib';
import { promisify } from 'node:util';
import { buildIngestEnvelope, parseCollectorResponse } from '../generated/spec-contract.ts';
import type { CollectorResponse } from '../generated/spec-contract.ts';
import { SDK_VERSION } from '../config/version.ts';

const gzipAsync = promisify(zlibGzip);

export interface CollectorClientOptions {
  url: string;
  apiKey?: string;
  authHeader?: string;
  timeout?: number;
  enableCompression?: boolean;
}

export interface VersionInfo {
  version: string;
  ingest_api_version: string;
  schema_version: string;
  event_version: string;
}

/** Standalone client for loxa-collector HTTP API. */
export class CollectorClient {
  private url: string;
  private apiKey: string;
  private authHeader: string;
  private timeout: number;
  private enableCompression: boolean;

  constructor(opts: CollectorClientOptions) {
    this.url = opts.url.replace(/\/$/, '');
    this.apiKey = opts.apiKey || '';
    this.authHeader = opts.authHeader || 'x-loxa-api-key';
    this.timeout = opts.timeout || 5000;
    this.enableCompression = opts.enableCompression ?? true;
  }

  /** Check if collector is healthy. */
  async health(): Promise<boolean> {
    try {
      const res = await this.get('/health');
      return res.statusCode === 200;
    } catch {
      return false;
    }
  }

  /** Check if collector is ready. */
  async ready(): Promise<boolean> {
    try {
      const res = await this.get('/ready');
      return res.statusCode === 200;
    } catch {
      return false;
    }
  }

  /** Get collector version info. */
  async version(): Promise<VersionInfo> {
    const res = await this.get('/version');
    return JSON.parse(res.body);
  }

  /** Send a batch of events to the collector. */
  async sendBatch(events: Record<string, any>[]): Promise<CollectorResponse> {
    const envelope = buildIngestEnvelope('loxa-js', SDK_VERSION, '', events);
    let body = Buffer.from(JSON.stringify(envelope), 'utf-8');

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };

    if (this.enableCompression) {
      body = await gzipAsync(body);
      headers['Content-Encoding'] = 'gzip';
    }

    if (this.apiKey) headers[this.authHeader] = this.apiKey;

    const res = await this.request('POST', '/events', body, headers);
    return parseCollectorResponse(res.body);
  }

  // --- Collector Admin API methods ---

  /** Validate an event against the collector schema. */
  async validate(event: Record<string, any>): Promise<any> {
    const res = await this.request('POST', '/validate', Buffer.from(JSON.stringify(event), 'utf-8'));
    return JSON.parse(res.body);
  }

  /** Ingest events (alias for sendBatch). */
  async ingest(events: Record<string, any>[]): Promise<any> {
    return this.sendBatch(events);
  }

  /** Query events from the collector. */
  async query(query: Record<string, any>): Promise<any> {
    const res = await this.request('POST', '/query', Buffer.from(JSON.stringify(query), 'utf-8'));
    return JSON.parse(res.body);
  }

  /** Tail events from the collector (streaming stub). */
  async tail(query: Record<string, any>): Promise<any> {
    const res = await this.request('POST', '/tail', Buffer.from(JSON.stringify(query), 'utf-8'));
    return JSON.parse(res.body);
  }

  /** Delete events from the collector. */
  async delete(query: Record<string, any>): Promise<any> {
    const res = await this.request('DELETE', '/events', Buffer.from(JSON.stringify(query), 'utf-8'));
    return JSON.parse(res.body);
  }

  /** Replay events from the collector. */
  async replay(query: Record<string, any>): Promise<any> {
    const res = await this.request('POST', '/replay', Buffer.from(JSON.stringify(query), 'utf-8'));
    return JSON.parse(res.body);
  }

  /** List DLQ entries. */
  async dlqList(query?: Record<string, any>): Promise<any> {
    const res = await this.request('GET', `/dlq${query ? '?' + new URLSearchParams(query as any).toString() : ''}`);
    return JSON.parse(res.body);
  }

  /** Read a DLQ entry by ID. */
  async dlqRead(id: string): Promise<any> {
    const res = await this.request('GET', `/dlq/${encodeURIComponent(id)}`);
    return JSON.parse(res.body);
  }

  /** Replay a DLQ entry by ID. */
  async dlqReplay(id: string): Promise<any> {
    const res = await this.request('POST', `/dlq/${encodeURIComponent(id)}/replay`);
    return JSON.parse(res.body);
  }

  /** Create an API key. */
  async keysCreate(name: string, scopes?: string[]): Promise<any> {
    const body = JSON.stringify({ name, scopes: scopes || ['ingest'] });
    const res = await this.request('POST', '/keys', Buffer.from(body, 'utf-8'));
    return JSON.parse(res.body);
  }

  /** Revoke an API key. */
  async keysRevoke(id: string): Promise<any> {
    const res = await this.request('POST', `/keys/${encodeURIComponent(id)}/revoke`);
    return JSON.parse(res.body);
  }

  /** Rotate an API key. */
  async keysRotate(id: string): Promise<any> {
    const res = await this.request('POST', `/keys/${encodeURIComponent(id)}/rotate`);
    return JSON.parse(res.body);
  }

  /** List configured sinks. */
  async sinksList(): Promise<any> {
    const res = await this.request('GET', '/sinks');
    return JSON.parse(res.body);
  }

  /** Test a configured sink. */
  async sinksTest(name: string): Promise<any> {
    const res = await this.request('POST', `/sinks/${encodeURIComponent(name)}/test`);
    return JSON.parse(res.body);
  }

  /** Validate an event governance policy. */
  async policyValidate(policy: Record<string, any>): Promise<any> {
    const res = await this.request('POST', '/policy/validate', Buffer.from(JSON.stringify(policy), 'utf-8'));
    return JSON.parse(res.body);
  }

  /** Check an event against the active schema. */
  async schemaCheck(event: Record<string, any>): Promise<any> {
    const res = await this.request('POST', '/schema/check', Buffer.from(JSON.stringify(event), 'utf-8'));
    return JSON.parse(res.body);
  }

  /** Publish schema metadata. */
  async schemaPublish(schema: Record<string, any>): Promise<any> {
    const res = await this.request('POST', '/schema/publish', Buffer.from(JSON.stringify(schema), 'utf-8'));
    return JSON.parse(res.body);
  }

  /** Apply retention policy immediately. */
  async retentionApply(policy?: Record<string, any>): Promise<any> {
    const res = await this.request('POST', '/retention/apply', Buffer.from(JSON.stringify(policy || {}), 'utf-8'));
    return JSON.parse(res.body);
  }

  private get(path: string): Promise<{ statusCode: number; body: string }> {
    return this.request('GET', path);
  }

  private request(method: string, path: string, body?: Buffer, extraHeaders?: Record<string, string>): Promise<{ statusCode: number; body: string }> {
    return new Promise((resolve, reject) => {
      const url = new URL(`${this.url}${path}`);
      const isHttps = url.protocol === 'https:';
      const mod = isHttps ? https : http;

      const headers: Record<string, string> = { ...extraHeaders };
      if (this.apiKey && !headers[this.authHeader]) headers[this.authHeader] = this.apiKey;
      if (body) headers['Content-Length'] = body.length.toString();

      const req = mod.request({
        hostname: url.hostname,
        port: url.port || (isHttps ? 443 : 80),
        path: url.pathname,
        method,
        headers,
        timeout: this.timeout,
      }, (res) => {
        let data = '';
        res.on('data', (chunk: Buffer) => { data += chunk.toString(); });
        res.on('end', () => {
          resolve({ statusCode: res.statusCode || 0, body: data });
        });
      });

      req.on('error', reject);
      req.on('timeout', () => { req.destroy(); reject(new Error('timeout')); });
      if (body) req.write(body);
      req.end();
    });
  }
}
