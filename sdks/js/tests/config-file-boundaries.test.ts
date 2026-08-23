import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { loadFileConfig, mergeFileConfig, parseSimpleYAML } from '../src/config/config-file.ts';
import { defaultConfig } from '../src/config/config.ts';

describe('Config file boundaries', () => {
  it('parses malformed, nested, quoted, and scalar YAML values', () => {
    assert.deepEqual(parseSimpleYAML('invalid\nkey: -3\ntruth: TRUE\nquoted: "value"\nempty:\n  nested: false\n'), {
      key: -3,
      truth: true,
      quoted: 'value',
      empty: { nested: false },
    });
    assert.deepEqual(parseSimpleYAML('a: one # comment\nb: two\n  child: value\nc: 3\n'), {
      a: 'one', b: 'two', child: 'value', c: 3,
    });
  });

  it('loads defaults and user overrides from explicit environment paths', () => {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'loza-config-'));
    const defaultsPath = path.join(directory, 'defaults.yaml');
    const userPath = path.join(directory, 'user.yaml');
    const previousDefaults = process.env.LOZA_JS_DEFAULTS;
    const previousConfig = process.env.LOZA_JS_CONFIG;
    try {
      fs.writeFileSync(defaultsPath, 'service_name: defaults\nasync:\n  workers: 1\nsecurity:\n  max_event_bytes: 100\n');
      fs.writeFileSync(userPath, 'service_name: user\nasync:\n  queue_size: 10\nsecurity:\n  max_event_bytes: 200\n');
      process.env.LOZA_JS_DEFAULTS = defaultsPath;
      process.env.LOZA_JS_CONFIG = userPath;
      assert.deepEqual(loadFileConfig(), {
        service_name: 'user', async: { workers: 1, queue_size: 10 }, security: { max_event_bytes: 200 },
      });
    } finally {
      if (previousDefaults === undefined) delete process.env.LOZA_JS_DEFAULTS;
      else process.env.LOZA_JS_DEFAULTS = previousDefaults;
      if (previousConfig === undefined) delete process.env.LOZA_JS_CONFIG;
      else process.env.LOZA_JS_CONFIG = previousConfig;
      fs.rmSync(directory, { recursive: true, force: true });
    }
  });

  it('applies all scalar, async, security, DSN, and max-buffer overlays', () => {
    const cfg = mergeFileConfig(defaultConfig(), {
      collector_url: 'loza://localhost:9308/project?env=staging&service=payments',
      service_name: 'service', service: 'override', service_version: '2', version: '3',
      environment: 'production', release: 'release', namespace: 'namespace', api_key: 'key', level: 'debug', duplicate_policy: 'drop',
      batch_size: 2, flush_interval: 3, max_retries: 4, max_backoff: 5, timeout: 6,
      strict: true, enable_compression: false, include_host: false, include_runtime: false,
      async: {
        enabled: true, queue_size: 10, queueSize: 11, workers: 2,
        max_batch_bytes: 12, maxBatchBytes: 13, flush_interval_ms: 14, flushIntervalMs: 15,
      },
      async_config: { enabled: false, workers: 3 },
      security: {
        redact_by_default: false, redactByDefault: true, allow_pii: true, allowPII: false,
        max_field_bytes: 10, maxFieldBytes: 11, max_event_bytes: 12, maxEventBytes: 13,
        max_attr_count: 14, maxAttrCount: 15, drop_oversized_events: false, dropOversizedEvents: true,
      },
      max_buffer_size: 99,
    });
    assert.equal(cfg.service, 'override');
    assert.equal(cfg.version, '3');
    assert.equal(cfg.collectorName, 'project');
    assert.equal(cfg.environment, 'production');
    assert.equal(cfg.async.queueSize, 99);
    assert.equal(cfg.async.workers, 2);
    assert.equal(cfg.async.maxBatchBytes, 13);
    assert.equal(cfg.async.flushIntervalMs, 15);
    assert.equal(cfg.security.maxFieldBytes, 11);
    assert.equal(cfg.security.maxEventBytes, 13);
    assert.equal(cfg.security.maxAttrCount, 15);
    assert.equal(cfg.security.dropOversizedEvents, true);
  });
  it('covers fallback file locations and unreadable config files', () => {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'loza-config-fallback-'));
    const previousCwd = process.cwd();
    const previousDefaults = process.env.LOZA_JS_DEFAULTS;
    const previousConfig = process.env.LOZA_JS_CONFIG;
    try {
      process.chdir(directory);
      delete process.env.LOZA_JS_DEFAULTS;
      delete process.env.LOZA_JS_CONFIG;
      fs.writeFileSync(path.join(directory, 'loza-js.defaults.yaml'), 'service_name: cwd-default\n');
      fs.writeFileSync(path.join(directory, '.loza-js.yaml'), 'service_name: dot-user\n');
      assert.equal(loadFileConfig().service_name, 'dot-user');
      fs.unlinkSync(path.join(directory, '.loza-js.yaml'));
      fs.unlinkSync(path.join(directory, 'loza-js.defaults.yaml'));
      assert.equal(loadFileConfig().collector_url, 'loza://localhost:9308/default');
      process.env.LOZA_JS_DEFAULTS = directory;
      process.env.LOZA_JS_CONFIG = directory;
      assert.deepEqual(loadFileConfig(), {});
    } finally {
      process.chdir(previousCwd);
      if (previousDefaults === undefined) delete process.env.LOZA_JS_DEFAULTS;
      else process.env.LOZA_JS_DEFAULTS = previousDefaults;
      if (previousConfig === undefined) delete process.env.LOZA_JS_CONFIG;
      else process.env.LOZA_JS_CONFIG = previousConfig;
      fs.rmSync(directory, { recursive: true, force: true });
    }
  });

  it('covers scalar skips, invalid DSNs, alternate nested sections, and array values', () => {
    const base = defaultConfig();
    const unchanged = mergeFileConfig(base, {
      collector_url: 'not-a-dsn',
      service_name: '',
      batch_size: 'not-number',
      strict: 'not-boolean',
      async: ['invalid'],
      async_config: { enabled: false, queue_size: 7 },
      security: ['invalid'],
      max_buffer_size: Number.NaN,
      ignored: null,
    });
    assert.equal(unchanged.collectorUrl, 'not-a-dsn');
    assert.equal(unchanged.async.queueSize, base.async.queueSize);
    const alternate = mergeFileConfig(defaultConfig(), { async_config: { enabled: false, queue_size: 7 } });
    assert.equal(alternate.async.queueSize, 7);
    const invalidDsn = mergeFileConfig(defaultConfig(), { collector_url: 'loza://%' });
    assert.equal(invalidDsn.collectorUrl, 'loza://%');
    const invalidNested = mergeFileConfig(defaultConfig(), { async: 1, security: 1 });
    assert.equal(invalidNested.async.queueSize, defaultConfig().async.queueSize);
  });
});
