/**
 * YAML-based config file loading for the JS/TS SDK.
 *
 * Implements the 4-layer config precedence:
 *   1. loza-js.defaults.yaml  (committed SDK defaults)
 *   2. .loza-js.yaml / loza.yaml  (user overrides)
 *   3. Environment variables (including LOZA_DSN)
 *   4. Code-level config (builder)
 *
 * Uses a simple built-in YAML parser (flat key: value + one level of nesting)
 * to avoid external dependencies.
 */

import { existsSync, readFileSync } from 'node:fs';
import { resolve, join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { Config } from './config.ts';
import { parse as parseDSN } from './dsn.ts';

// ── Simple YAML parser ──────────────────────────────────────────────────────

/** Parsed YAML as a nested plain object. */
export type YamlRecord = Record<string, unknown>;

/**
 * Parse simple YAML content into a plain object.
 * Handles flat key: value pairs and one level of nesting.
 * Comments (# ...) and blank lines are skipped.
 * Values are parsed as booleans, integers, or strings.
 */
export function parseSimpleYAML(content: string): YamlRecord {
  const root: YamlRecord = {};
  const stack: Array<{ indent: number; obj: YamlRecord }> = [{ indent: -1, obj: root }];

  for (const rawLine of content.split('\n')) {
    const trimmed = rawLine.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;

    const indent = rawLine.length - rawLine.trimStart().length;

    // Pop stack until we find the parent for this indent level
    while (stack.length > 1 && indent <= stack[stack.length - 1].indent) {
      stack.pop();
    }

    const colonIdx = trimmed.indexOf(':');
    if (colonIdx === -1) continue;

    const key = trimmed.slice(0, colonIdx).trim();
    const rawValue = trimmed.slice(colonIdx + 1).trim();

    // Strip inline comments
    let value = rawValue;
    const commentIdx = value.indexOf('#');
    if (commentIdx > 0) {
      value = value.slice(0, commentIdx).trim();
    }

    const current = stack[stack.length - 1].obj;

    if (value === '') {
      // Empty value means nested object
      const child: YamlRecord = {};
      current[key] = child;
      stack.push({ indent, obj: child });
    } else {
      current[key] = parseScalar(value);
    }
  }

  return root;
}

/** Parse a YAML scalar value (string, boolean, integer). */
function parseScalar(value: string): unknown {
  // Strip surrounding quotes
  if (
    value.length >= 2 &&
    value[0] === value[value.length - 1] &&
    (value[0] === '"' || value[0] === "'")
  ) {
    value = value.slice(1, -1);
  }

  const lower = value.toLowerCase();
  if (lower === 'true') return true;
  if (lower === 'false') return false;

  // Integer
  if (/^-?\d+$/.test(value)) {
    const n = parseInt(value, 10);
    if (!isNaN(n)) return n;
  }

  return value;
}

// ── Path resolution ─────────────────────────────────────────────────────────

/** Resolve the package root directory (two levels up from this file). */
function getPackageRoot(): string {
  try {
    const here = dirname(fileURLToPath(import.meta.url));
    return resolve(here, '..', '..');
  } catch {
    return process.cwd();
  }
}

/**
 * Find the path to loza-js.defaults.yaml.
 * Search order: LOZA_JS_DEFAULTS env, cwd, package root.
 */
function findDefaultsPath(): string | null {
  const envOverride = process.env.LOZA_JS_DEFAULTS?.trim();
  if (envOverride && existsSync(envOverride)) return envOverride;

  const cwdCandidate = join(process.cwd(), 'loza-js.defaults.yaml');
  if (existsSync(cwdCandidate)) return cwdCandidate;

  const pkgCandidate = join(getPackageRoot(), 'loza-js.defaults.yaml');
  if (existsSync(pkgCandidate)) return pkgCandidate;

  return null;
}

/**
 * Find the path to user config file.
 * Search order: LOZA_JS_CONFIG env, .loza-js.yaml in cwd, loza.yaml in cwd.
 */
function findUserConfigPath(): string | null {
  const envOverride = process.env.LOZA_JS_CONFIG?.trim();
  if (envOverride && existsSync(envOverride)) return envOverride;

  const dotCandidate = join(process.cwd(), '.loza-js.yaml');
  if (existsSync(dotCandidate)) return dotCandidate;

  const lozaCandidate = join(process.cwd(), 'loza.yaml');
  if (existsSync(lozaCandidate)) return lozaCandidate;

  return null;
}

// ── Overlay ─────────────────────────────────────────────────────────────────

/** Deep-merge override onto base (override wins for non-empty values). */
function overlayRaw(base: YamlRecord, override: YamlRecord): YamlRecord {
  const result: YamlRecord = { ...base };
  for (const [key, value] of Object.entries(override)) {
    if (value === undefined || value === null || value === '') continue;
    if (
      typeof value === 'object' &&
      !Array.isArray(value) &&
      typeof result[key] === 'object' &&
      result[key] !== null &&
      !Array.isArray(result[key])
    ) {
      result[key] = overlayRaw(
        result[key] as YamlRecord,
        value as YamlRecord,
      );
    } else {
      result[key] = value;
    }
  }
  return result;
}

// ── Public API ──────────────────────────────────────────────────────────────

/**
 * Load and overlay YAML config files (defaults + user override).
 * Returns the merged raw YAML dict, or empty object if no files are found.
 * Only works in Node.js environments (silently returns {} in browsers).
 */
export function loadFileConfig(): YamlRecord {
  if (typeof process === 'undefined') return {};

  let raw: YamlRecord = {};

  // Layer 1: SDK defaults
  const defaultsPath = findDefaultsPath();
  if (defaultsPath) {
    try {
      const content = readFileSync(defaultsPath, 'utf-8');
      raw = parseSimpleYAML(content);
    } catch {
      // Read failed, continue with empty
    }
  }

  // Layer 2: User overrides (overlay on top of defaults)
  const userPath = findUserConfigPath();
  if (userPath) {
    try {
      const content = readFileSync(userPath, 'utf-8');
      const userRaw = parseSimpleYAML(content);
      raw = overlayRaw(raw, userRaw);
    } catch {
      // Read failed, continue with defaults only
    }
  }

  return raw;
}

/**
 * Apply raw YAML config values to a Config object.
 * Only non-empty YAML values are applied (empty/missing fields are skipped).
 * If collector_url is a loza:// DSN, it is parsed and resolved to an HTTP URL.
 */
export function mergeFileConfig(base: Config, raw: YamlRecord): Config {
  const cfg = { ...base };

  // ── Top-level scalar fields ────────────────────────────────────────────────

  applyString(raw, cfg, 'collector_url', 'collectorUrl');
  applyString(raw, cfg, 'service_name', 'service');
  applyString(raw, cfg, 'service', 'service');
  applyString(raw, cfg, 'service_version', 'version');
  applyString(raw, cfg, 'version', 'version');
  applyString(raw, cfg, 'environment', 'environment');
  applyString(raw, cfg, 'release', 'release');
  applyString(raw, cfg, 'namespace', 'namespace');
  applyString(raw, cfg, 'api_key', 'apiKey');
  applyString(raw, cfg, 'level', 'level');
  applyString(raw, cfg, 'duplicate_policy', 'duplicatePolicy');

  applyNumber(raw, cfg, 'batch_size', 'batchSize');
  applyNumber(raw, cfg, 'flush_interval', 'flushIntervalMs');
  // max_buffer_size maps to async.queueSize (applied after nested section)
  applyNumber(raw, cfg, 'max_retries', 'maxRetries');
  applyNumber(raw, cfg, 'max_backoff', 'maxBackoffMs');
  applyNumber(raw, cfg, 'timeout', 'timeoutMs');

  applyBoolean(raw, cfg, 'strict', 'strict');
  applyBoolean(raw, cfg, 'enable_compression', 'enableCompression');
  applyBoolean(raw, cfg, 'include_host', 'includeHost');
  applyBoolean(raw, cfg, 'include_runtime', 'includeRuntime');

  // ── Parse collector_url as DSN if it uses loza:// scheme ───────────────────

  if (cfg.collectorUrl && cfg.collectorUrl.startsWith('loza://')) {
    try {
      const dsn = parseDSN(cfg.collectorUrl);
      cfg.collectorUrl = dsn.baseURL;
      cfg.collectorName = dsn.collectorName;
      if (dsn.username !== undefined) {
        cfg.username = dsn.username;
        cfg.password = dsn.password ?? '';
      }
      // Extract DSN-derived values (only override if not already set from YAML)
      if (dsn.env && dsn.env !== 'default' && cfg.environment === 'development') {
        cfg.environment = dsn.env;
      }
      if (dsn.service && !cfg.service) {
        cfg.service = dsn.service;
      }
    } catch {
      // Invalid DSN — leave as-is, env vars or code config may fix it
    }
  }

  // ── Nested: async ──────────────────────────────────────────────────────────

  const asyncRaw = raw.async ?? raw.async_config;
  if (asyncRaw && typeof asyncRaw === 'object' && !Array.isArray(asyncRaw)) {
    const a = asyncRaw as YamlRecord;
    const async = { ...cfg.async };
    if (a.enabled !== undefined) async.enabled = Boolean(a.enabled);
    if (a.queue_size !== undefined && typeof a.queue_size === 'number') async.queueSize = a.queue_size;
    if (a.queueSize !== undefined && typeof a.queueSize === 'number') async.queueSize = a.queueSize;
    if (a.workers !== undefined && typeof a.workers === 'number') async.workers = a.workers;
    if (a.max_batch_bytes !== undefined && typeof a.max_batch_bytes === 'number') async.maxBatchBytes = a.max_batch_bytes;
    if (a.maxBatchBytes !== undefined && typeof a.maxBatchBytes === 'number') async.maxBatchBytes = a.maxBatchBytes;
    if (a.flush_interval_ms !== undefined && typeof a.flush_interval_ms === 'number') async.flushIntervalMs = a.flush_interval_ms;
    if (a.flushIntervalMs !== undefined && typeof a.flushIntervalMs === 'number') async.flushIntervalMs = a.flushIntervalMs;
    cfg.async = async;
  }

  // ── Nested: security ──────────────────────────────────────────────────────

  const secRaw = raw.security;
  if (secRaw && typeof secRaw === 'object' && !Array.isArray(secRaw)) {
    const s = secRaw as YamlRecord;
    const security = { ...cfg.security };
    if (s.redact_by_default !== undefined) security.redactByDefault = Boolean(s.redact_by_default);
    if (s.redactByDefault !== undefined) security.redactByDefault = Boolean(s.redactByDefault);
    if (s.allow_pii !== undefined) security.allowPII = Boolean(s.allow_pii);
    if (s.allowPII !== undefined) security.allowPII = Boolean(s.allowPII);
    if (s.max_field_bytes !== undefined && typeof s.max_field_bytes === 'number') security.maxFieldBytes = s.max_field_bytes;
    if (s.maxFieldBytes !== undefined && typeof s.maxFieldBytes === 'number') security.maxFieldBytes = s.maxFieldBytes;
    if (s.max_event_bytes !== undefined && typeof s.max_event_bytes === 'number') security.maxEventBytes = s.max_event_bytes;
    if (s.maxEventBytes !== undefined && typeof s.maxEventBytes === 'number') security.maxEventBytes = s.maxEventBytes;
    if (s.max_attr_count !== undefined && typeof s.max_attr_count === 'number') security.maxAttrCount = s.max_attr_count;
    if (s.maxAttrCount !== undefined && typeof s.maxAttrCount === 'number') security.maxAttrCount = s.maxAttrCount;
    if (s.drop_oversized_events !== undefined) security.dropOversizedEvents = Boolean(s.drop_oversized_events);
    if (s.dropOversizedEvents !== undefined) security.dropOversizedEvents = Boolean(s.dropOversizedEvents);
    cfg.security = security;
  }

  // ── Top-level max_buffer_size -> async.queueSize ───────────────────────────
  // The loza-js.defaults.yaml uses max_buffer_size at the top level,
  // which maps to async.queueSize in the JS Config interface.

  const maxBuf = raw.max_buffer_size;
  if (typeof maxBuf === 'number' && !isNaN(maxBuf)) {
    cfg.async = { ...cfg.async, queueSize: maxBuf };
  }

  return cfg;
}

// ── Helpers ─────────────────────────────────────────────────────────────────

/** Apply a string YAML value to a Config field. */
function applyString(
  raw: YamlRecord,
  cfg: Config,
  yamlKey: string,
  cfgKey: keyof Config,
): void {
  const value = raw[yamlKey];
  if (typeof value === 'string' && value !== '') {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (cfg as any)[cfgKey] = value;
  }
}

/** Apply a numeric YAML value to a Config field. */
function applyNumber(
  raw: YamlRecord,
  cfg: Config,
  yamlKey: string,
  cfgKey: keyof Config,
): void {
  const value = raw[yamlKey];
  if (typeof value === 'number' && !isNaN(value)) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (cfg as any)[cfgKey] = value;
  }
}

/** Apply a boolean YAML value to a Config field. */
function applyBoolean(
  raw: YamlRecord,
  cfg: Config,
  yamlKey: string,
  cfgKey: keyof Config,
): void {
  const value = raw[yamlKey];
  if (typeof value === 'boolean') {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (cfg as any)[cfgKey] = value;
  }
}
