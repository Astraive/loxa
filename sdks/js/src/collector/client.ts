import http from 'node:http';
import https from 'node:https';
import { gzip as zlibGzip } from 'node:zlib';
import { promisify } from 'node:util';
import { buildIngestEnvelope, parseCollectorResponse } from '../generated/spec-contract.ts';
import type { CollectorResponse } from '../generated/spec-contract.ts';

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
    const envelope = buildIngestEnvelope('loxa-js', '1.0.0', '', events);
    let body = Buffer.from(JSON.stringify(envelope), 'utf-8');

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };

    if (this.enableCompression) {
      body = await gzipAsync(body);
      headers['Content-Encoding'] = 'gzip';
    }

    if (this.apiKey) headers[this.authHeader] = this.apiKey;

    const res = await this.request('POST', '/v1/events', body, headers);
    return parseCollectorResponse(res.body);
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
