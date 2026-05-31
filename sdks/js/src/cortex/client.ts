import http from 'node:http';
import https from 'node:https';

export interface CortexClientOptions {
  url: string;
  apiKey?: string;
  timeout?: number;
  maxResponseBytes?: number;
}

export interface ReconstructionResult {
  incident_id: string;
  timestamp: string;
  related_events: any[];
  causal_chain: any[];
  similar_past_incidents: any[];
  suggested_remediations: any[];
  related_services: string[];
  symptoms: any[];
  confidence: number;
  explain: string;
}

export interface GraphResult {
  nodes: any[];
  edges: any[];
}

export interface Remediation {
  remediation_id?: string;
  incident_id: string;
  action: string;
  timestamp?: string;
  operator?: string;
  attributes?: Record<string, any>;
}

export interface Feedback {
  feedback_id?: string;
  remediation_id?: string;
  incident_id: string;
  outcome: string;
  time_to_resolve_seconds?: number;
  timestamp?: string;
  notes?: string;
}

// --- Normalization ---

function normalizeTimestamp(ts: string): string {
  ts = (ts || '').trim();
  return ts || new Date().toISOString();
}

/** Normalize an IncidentContext: trim strings, clamp confidence, fill empty timestamps. */
export function normalizeIncidentContext(ctx: any): void {
  if (!ctx) return;
  ctx.incident_id = (ctx.incident_id || '').trim();
  ctx.timestamp = normalizeTimestamp(ctx.timestamp);
  if (typeof ctx.confidence === 'number') {
    ctx.confidence = Math.max(0, Math.min(1, ctx.confidence));
  }
  if (Array.isArray(ctx.related_services)) {
    ctx.related_services = ctx.related_services.map((s: string) => (s || '').trim());
  }
}

/** Normalize GraphView: trim node IDs. */
export function normalizeGraphView(gv: any): void {
  if (!gv) return;
  for (const node of gv.nodes || []) {
    if (typeof node.id === 'string') node.id = node.id.trim();
  }
}

/** Normalize Remediation: trim strings, normalize timestamp. */
export function normalizeRemediation(r: any): void {
  if (!r) return;
  r.remediation_id = (r.remediation_id || '').trim();
  r.incident_id = (r.incident_id || '').trim();
  r.action = (r.action || '').trim();
  r.operator = (r.operator || '').trim();
  r.timestamp = normalizeTimestamp(r.timestamp);
}

// --- Validation ---

/** Validate IncidentContext has required fields. */
export function validateIncidentContext(ctx: any): void {
  if (!ctx) throw new Error('cortex: incident context is nil');
  if (!ctx.incident_id) throw new Error('cortex: incident_id is required');
  if (!ctx.timestamp) throw new Error('cortex: timestamp is required');
  if (typeof ctx.confidence === 'number' && (ctx.confidence < 0 || ctx.confidence > 1)) {
    throw new Error(`cortex: confidence must be in [0.0, 1.0], got ${ctx.confidence}`);
  }
}

/** Validate GraphView has valid structure. */
export function validateGraphView(gv: any): void {
  if (!gv) throw new Error('cortex: graph view is nil');
  for (let i = 0; i < (gv.nodes || []).length; i++) {
    if (!gv.nodes[i].id) throw new Error(`cortex: node ${i} missing 'id'`);
  }
  for (let i = 0; i < (gv.edges || []).length; i++) {
    const edge = gv.edges[i];
    if (!edge.source && !edge.from_node_id) {
      throw new Error(`cortex: edge ${i} missing 'source' or 'from_node_id'`);
    }
    if (!edge.target && !edge.to_node_id) {
      throw new Error(`cortex: edge ${i} missing 'target' or 'to_node_id'`);
    }
  }
}

/** Validate Remediation has required fields. */
export function validateRemediation(r: any): void {
  if (!r) throw new Error('cortex: remediation is nil');
  if (!r.incident_id) throw new Error('cortex: incident_id is required');
  if (!r.action) throw new Error('cortex: action is required');
}

/** Validate Feedback has required fields. */
export function validateFeedback(f: any): void {
  if (!f) throw new Error('cortex: feedback is nil');
  if (!f.incident_id) throw new Error('cortex: incident_id is required');
  if (!f.outcome) throw new Error('cortex: outcome is required');
}

/** Client for loxa-cortex HTTP API. */
export class CortexClient {
  private url: string;
  private apiKey: string;
  private timeout: number;
  private maxResponseBytes: number;

  constructor(opts: CortexClientOptions) {
    this.url = opts.url.replace(/\/$/, '');
    this.apiKey = opts.apiKey || '';
    this.timeout = opts.timeout || 5000;
    this.maxResponseBytes = opts.maxResponseBytes || 10 * 1024 * 1024;
  }

  /** Check if cortex is healthy. */
  async health(): Promise<boolean> {
    try {
      const res = await this.get('/healthz');
      return res.statusCode === 200;
    } catch {
      return false;
    }
  }

  /** Check if cortex is ready. */
  async ready(): Promise<boolean> {
    try {
      const res = await this.get('/readyz');
      return res.statusCode === 200;
    } catch {
      return false;
    }
  }

  /** Ingest events into cortex. */
  async ingestBatch(events: Record<string, any>[]): Promise<void> {
    await this.post('/events/batch', { events });
  }

  /** Ingest events as NDJSON. */
  async ingestJsonl(events: Record<string, any>[]): Promise<void> {
    const body = events.map(e => JSON.stringify(e)).join('\n');
    await this.request('POST', '/events/jsonl', body, 'application/x-ndjson');
  }

  /** Reconstruct incident context. */
  async reconstruct(incidentId: string, mode: string = 'fast'): Promise<ReconstructionResult> {
    const res = await this.post('/reconstruct', { incident_id: incidentId, mode });
    const result = JSON.parse(res.body);
    normalizeIncidentContext(result);
    validateIncidentContext(result);
    return result;
  }

  /** Find incidents similar to the given one. */
  async similarIncidents(incidentId: string, limit: number = 10): Promise<any[]> {
    const res = await this.post('/reconstruct', { incident_id: incidentId, mode: 'fast' });
    const data = JSON.parse(res.body);
    return (data.similar_incidents || data.similar_past_incidents || []).slice(0, limit);
  }

  /** Get service dependency graph. */
  async serviceGraph(service: string): Promise<GraphResult> {
    const res = await this.get(`/graph/service/${encodeURIComponent(service)}`);
    const result = JSON.parse(res.body);
    normalizeGraphView(result);
    validateGraphView(result);
    return result;
  }

  /** Get incident graph. */
  async incidentGraph(incidentId: string): Promise<GraphResult> {
    const res = await this.get(`/graph/incident/${encodeURIComponent(incidentId)}`);
    const result = JSON.parse(res.body);
    normalizeGraphView(result);
    validateGraphView(result);
    return result;
  }

  /** Record remediation action. */
  async recordRemediation(data: Remediation): Promise<void> {
    normalizeRemediation(data);
    validateRemediation(data);
    await this.post('/feedback/remediation', data);
  }

  /** Record incident feedback. */
  async recordFeedback(data: Feedback): Promise<void> {
    validateFeedback(data);
    await this.post('/feedback/incident', data);
  }

  private get(path: string): Promise<{ statusCode: number; body: string }> {
    return this.request('GET', path);
  }

  private post(path: string, body: any): Promise<{ statusCode: number; body: string }> {
    return this.request('POST', path, JSON.stringify(body));
  }

  private request(method: string, path: string, body?: string, contentType?: string): Promise<{ statusCode: number; body: string }> {
    return new Promise((resolve, reject) => {
      const url = new URL(`${this.url}${path}`);
      const isHttps = url.protocol === 'https:';
      const mod = isHttps ? https : http;

      const headers: Record<string, string> = {};
      if (this.apiKey) headers['x-api-key'] = this.apiKey;
      if (body) {
        headers['Content-Type'] = contentType || 'application/json';
        headers['Content-Length'] = Buffer.byteLength(body).toString();
      }

      const req = mod.request({
        hostname: url.hostname,
        port: url.port || (isHttps ? 443 : 80),
        path: `${url.pathname}${url.search}`,
        method,
        headers,
        timeout: this.timeout,
      }, (res) => {
        let data = '';
        let bytes = 0;
        res.on('data', (chunk: Buffer) => {
          bytes += chunk.length;
          if (bytes > this.maxResponseBytes) {
            req.destroy(new Error(`response exceeds ${this.maxResponseBytes} bytes`));
            return;
          }
          data += chunk.toString();
        });
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
