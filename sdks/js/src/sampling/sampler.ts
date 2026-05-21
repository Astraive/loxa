import { Event } from '../core/event.ts';

/** Sampler function type — returns true to keep, false to drop. */
export type Sampler = (event: Event) => boolean;

/** Keep every event. */
export function sampleAll(): Sampler {
  return () => true;
}

/** Drop every event. */
export function sampleNone(): Sampler {
  return () => false;
}

/** Keep approximately `rate` fraction (0.0 to 1.0). */
export function sampleRandom(rate: number): Sampler {
  return () => Math.random() < rate;
}

/** Keep only error events. */
export function sampleErrors(): Sampler {
  return (ev) => ev.outcome === 'error' || ev.error !== null;
}

/** Keep events with duration >= threshold (ms). */
export function sampleSlowRequests(thresholdMs: number): Sampler {
  return (ev) => ev.durationMs >= thresholdMs;
}

/** Keep events with matching HTTP status codes. */
export function sampleStatusCodes(...codes: number[]): Sampler {
  const set = new Set(codes);
  return (ev) => set.has(ev.statusCode);
}

/** Keep events matching route or path. */
export function sampleRoutes(...routes: string[]): Sampler {
  const set = new Set(routes);
  return (ev) => set.has(ev.route) || set.has(ev.path);
}

/** Keep events for specific user IDs. */
export function sampleUsers(...ids: string[]): Sampler {
  const set = new Set(ids);
  return (ev) => set.has(ev.attrs['user.id']);
}

/** Keep events for specific tenant IDs. */
export function sampleTenants(...ids: string[]): Sampler {
  const set = new Set(ids);
  return (ev) => set.has(ev.attrs['tenant.id']);
}

/** Keep events matching a feature flag value. */
export function sampleFeatureFlag(name: string, value: any): Sampler {
  return (ev) => ev.attrs[`feature.${name}`] === value;
}

/** Logical OR combinator. */
export function anySampler(...samplers: Sampler[]): Sampler {
  return (ev) => samplers.some(s => s(ev));
}

/** Logical AND combinator. */
export function allSampler(...samplers: Sampler[]): Sampler {
  return (ev) => samplers.every(s => s(ev));
}

/** Logical NOT combinator. */
export function notSampler(sampler: Sampler): Sampler {
  return (ev) => !sampler(ev);
}

/** Token-bucket rate limiter — keeps events up to `rate` per `windowMs`. */
export function sampleRateLimited(rate: number, windowMs: number = 1000): Sampler {
  let tokens = rate;
  let last = Date.now();
  return () => {
    const now = Date.now();
    const elapsed = now - last;
    last = now;
    tokens = Math.min(rate, tokens + elapsed * (rate / windowMs));
    if (tokens < 1) return false;
    tokens -= 1;
    return true;
  };
}

/** Keep events where an HTTP header attr matches value (or is present if value empty). */
export function sampleByHeader(header: string, value?: string): Sampler {
  const lower = header.toLowerCase().replace(/_/g, '-');
  return (ev) => {
    const v = ev.attrs[`http.header.${lower}`]
      ?? ev.attrs[`http.headers.${lower}`]
      ?? ev.attrs[lower];
    if (value === undefined || value === '') return v != null && v !== '';
    return String(v) === value;
  };
}
