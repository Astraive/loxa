import { createHash } from 'node:crypto';

/** Redactor function type — transforms a payload dict. */
export type Redactor = (payload: Record<string, any>) => Record<string, any>;

const REDACTED = '[REDACTED]';

/** Safety-net keys for obviously sensitive fields. Collector owns full PII policy. */
const DEFAULT_KEYS: Set<string> = new Set([
  'password', 'passwd', 'pwd', 'secret', 'token', 'access_token', 'refresh_token',
  'api_key', 'apikey', 'auth', 'authorization', 'credential', 'private_key', 'client_secret',
]);

/** Case-insensitive key match against a set.
 *  Checks the full key and every segment split by . _ or -.
 *  This handles both dotted paths (user.password) and compound names (credit_card_number). */
function matchesKey(key: string, keys: Set<string>): boolean {
  const lower = key.toLowerCase();
  if (keys.has(lower)) return true;
  // Collect all segments split by common delimiters
  const segments = lower.split(/[._-]+/);
  for (const segment of segments) {
    if (keys.has(segment)) return true;
  }
  return false;
}

/** Walk a payload recursively, applying a transform to matching keys. */
function walk(obj: any, keys: Set<string>, transform: (key: string, value: any) => [boolean, any]): any {
  if (Array.isArray(obj)) {
    return obj.map(item => walk(item, keys, transform));
  }
  if (obj && typeof obj === 'object' && obj.constructor === Object) {
    const out: Record<string, any> = {};
    for (const [k, v] of Object.entries(obj)) {
      const [keep, newVal] = transform(k, v);
      if (keep) {
        out[k] = walk(newVal, keys, transform);
      }
    }
    return out;
  }
  return obj;
}

/** Default redactor — replaces values of sensitive keys with [REDACTED]. */
export function defaultRedactor(): Redactor {
  return redactKeys(...DEFAULT_KEYS);
}

/** Redactor that replaces matching key values with [REDACTED]. */
export function redactKeys(...keys: string[]): Redactor {
  const keySet = new Set(keys.map(k => k.toLowerCase()));
  return (payload) => walk(payload, keySet, (key, value) => {
    if (matchesKey(key, keySet)) return [true, REDACTED];
    return [true, value];
  });
}

export function hashKeys(...keys: string[]): Redactor {
  const keySet = new Set(keys.map(k => k.toLowerCase()));
  return (payload) => walk(payload, keySet, (key, value) => {
    if (matchesKey(key, keySet)) {
      const hash = createHash('sha256').update(String(value)).digest('hex');
      return [true, `sha256:${hash}`];
    }
    return [true, value];
  });
}

/** Redactor that drops matching keys entirely. */
export function dropKeys(...keys: string[]): Redactor {
  const keySet = new Set(keys.map(k => k.toLowerCase()));
  return (payload) => walk(payload, keySet, (key, value) => {
    if (matchesKey(key, keySet)) return [false, value];
    return [true, value];
  });
}

/** Redactor that masks matching key values (first 2 + last 2 chars visible). */
export function maskKeys(...keys: string[]): Redactor {
  const keySet = new Set(keys.map(k => k.toLowerCase()));
  return (payload) => walk(payload, keySet, (key, value) => {
    if (matchesKey(key, keySet) && typeof value === 'string') {
      if (value.length <= 4) return [true, '****'];
      return [true, value.slice(0, 2) + '*'.repeat(value.length - 4) + value.slice(-2)];
    }
    return [true, value];
  });
}

/** Redactor that chains multiple redactors (first match wins per key). */
export function composeRedactors(...redactors: Redactor[]): Redactor {
  return (payload) => {
    let current = { ...payload };
    for (const r of redactors) {
      current = r(current);
    }
    return current;
  };
}

/** Redactor that replaces values of keys matching regex patterns with [REDACTED]. */
export function redactPatterns(...patterns: string[]): Redactor {
  const regexes = patterns.map(p => new RegExp(p, 'i'));
  return (payload) => walk(payload, new Set(), (key, value) => {
    if (regexes.some(r => r.test(key))) return [true, REDACTED];
    return [true, value];
  });
}
