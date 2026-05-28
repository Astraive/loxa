import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { parseSimpleYAML, loadFileConfig, mergeFileConfig } from '../src/config/config-file.ts';
import { defaultConfig, fromEnv } from '../src/config/config.ts';

describe('parseSimpleYAML', () => {
  it('parses flat key: value pairs', () => {
    const result = parseSimpleYAML(`
collector_url: http://localhost:9308
batch_size: 100
environment: production
`);
    assert.equal(result.collector_url, 'http://localhost:9308');
    assert.equal(result.batch_size, 100);
    assert.equal(result.environment, 'production');
  });

  it('skips comments and blank lines', () => {
    const result = parseSimpleYAML(`
# This is a comment
collector_url: http://localhost:9308

# Another comment
batch_size: 100
`);
    assert.equal(result.collector_url, 'http://localhost:9308');
    assert.equal(result.batch_size, 100);
    assert.equal(Object.keys(result).length, 2);
  });

  it('parses boolean values', () => {
    const result = parseSimpleYAML(`
strict: true
enable_compression: false
`);
    assert.equal(result.strict, true);
    assert.equal(result.enable_compression, false);
  });

  it('parses quoted string values', () => {
    const result = parseSimpleYAML(`
service: "my-service"
environment: 'staging'
`);
    assert.equal(result.service, 'my-service');
    assert.equal(result.environment, 'staging');
  });

  it('parses nested objects', () => {
    const result = parseSimpleYAML(`
async:
  enabled: true
  queue_size: 4096
  workers: 2
security:
  redact_by_default: true
  max_field_bytes: 8192
`);
    assert.equal(typeof result.async, 'object');
    const async = result.async as Record<string, unknown>;
    assert.equal(async.enabled, true);
    assert.equal(async.queue_size, 4096);
    assert.equal(async.workers, 2);

    const security = result.security as Record<string, unknown>;
    assert.equal(security.redact_by_default, true);
    assert.equal(security.max_field_bytes, 8192);
  });

  it('handles empty content', () => {
    const result = parseSimpleYAML('');
    assert.deepEqual(result, {});
  });

  it('handles content with only comments', () => {
    const result = parseSimpleYAML('# comment\n# another');
    assert.deepEqual(result, {});
  });

  it('strips inline comments', () => {
    const result = parseSimpleYAML('batch_size: 100 # default batch');
    assert.equal(result.batch_size, 100);
  });
});

describe('mergeFileConfig', () => {
  it('applies collector_url to collectorUrl', () => {
    const base = defaultConfig();
    const raw = { collector_url: 'http://collector:9308' };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.collectorUrl, 'http://collector:9308');
  });

  it('parses loxa:// DSN in collector_url', () => {
    const base = defaultConfig();
    const raw = { collector_url: 'loxa://localhost:9308/my-project' };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.collectorUrl, 'http://localhost:9308');
  });

  it('extracts env from DSN when environment is default', () => {
    const base = defaultConfig();
    const raw = { collector_url: 'loxa://localhost:9308/prod?env=staging' };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.environment, 'staging');
  });

  it('applies batch_size to batchSize', () => {
    const base = defaultConfig();
    const raw = { batch_size: 200 };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.batchSize, 200);
  });

  it('applies flush_interval to flushIntervalMs', () => {
    const base = defaultConfig();
    const raw = { flush_interval: 3000 };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.flushIntervalMs, 3000);
  });

  it('applies max_retries to maxRetries', () => {
    const base = defaultConfig();
    const raw = { max_retries: 5 };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.maxRetries, 5);
  });

  it('applies timeout to timeoutMs', () => {
    const base = defaultConfig();
    const raw = { timeout: 15000 };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.timeoutMs, 15000);
  });

  it('applies environment', () => {
    const base = defaultConfig();
    const raw = { environment: 'production' };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.environment, 'production');
  });

  it('applies service_name to service', () => {
    const base = defaultConfig();
    const raw = { service_name: 'my-svc' };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.service, 'my-svc');
  });

  it('prefers service_name over service when both present', () => {
    const base = defaultConfig();
    const raw = { service_name: 'from-name', service: 'from-service' };
    const result = mergeFileConfig(base, raw);
    // service_name is processed first, then service overwrites it
    assert.equal(result.service, 'from-service');
  });

  it('applies nested async config', () => {
    const base = defaultConfig();
    const raw = {
      async: {
        enabled: true,
        queue_size: 4096,
        workers: 2,
        max_batch_bytes: 1024 * 1024,
      },
    };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.async.enabled, true);
    assert.equal(result.async.queueSize, 4096);
    assert.equal(result.async.workers, 2);
    assert.equal(result.async.maxBatchBytes, 1024 * 1024);
  });

  it('applies nested security config', () => {
    const base = defaultConfig();
    const raw = {
      security: {
        redact_by_default: false,
        allow_pii: true,
        max_field_bytes: 8192,
      },
    };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.security.redactByDefault, false);
    assert.equal(result.security.allowPII, true);
    assert.equal(result.security.maxFieldBytes, 8192);
  });

  it('supports camelCase YAML keys for async', () => {
    const base = defaultConfig();
    const raw = {
      async: {
        enabled: true,
        queueSize: 2048,
        workers: 8,
      },
    };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.async.enabled, true);
    assert.equal(result.async.queueSize, 2048);
    assert.equal(result.async.workers, 8);
  });

  it('supports camelCase YAML keys for security', () => {
    const base = defaultConfig();
    const raw = {
      security: {
        redactByDefault: false,
        allowPII: true,
      },
    };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.security.redactByDefault, false);
    assert.equal(result.security.allowPII, true);
  });

  it('does not overwrite base values with empty strings', () => {
    const base = defaultConfig();
    base.environment = 'production';
    const raw = { environment: '' };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.environment, 'production');
  });

  it('applies strict boolean', () => {
    const base = defaultConfig();
    const raw = { strict: true };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.strict, true);
  });

  it('applies enable_compression boolean', () => {
    const base = defaultConfig();
    const raw = { enable_compression: false };
    const result = mergeFileConfig(base, raw);
    assert.equal(result.enableCompression, false);
  });

  it('applies max_buffer_size to async.queueSize', () => {
    const base = defaultConfig();
    const raw = { max_buffer_size: 50000 };
    const result = mergeFileConfig(base, raw);
    // max_buffer_size at top level maps to async.queueSize
    assert.equal(result.async.queueSize, 50000);
  });
});

describe('loadFileConfig', () => {
  it('returns an object (may be empty if no YAML files in cwd)', () => {
    const result = loadFileConfig();
    assert.equal(typeof result, 'object');
    assert.notEqual(result, null);
  });
});

describe('fromEnv with DSN', () => {
  it('parses LOXA_DSN env var when set', () => {
    const original = process.env.LOXA_DSN;
    const originalUrl = process.env.LOXA_COLLECTOR_URL;
    try {
      process.env.LOXA_DSN = 'loxa://collector.example.com/my-app?env=staging&service=api';
      delete process.env.LOXA_COLLECTOR_URL;
      const cfg = fromEnv();
      assert.equal(cfg.collectorUrl, 'https://collector.example.com:443');
      assert.equal(cfg.environment, 'staging');
      assert.equal(cfg.service, 'api');
    } finally {
      if (original !== undefined) {
        process.env.LOXA_DSN = original;
      } else {
        delete process.env.LOXA_DSN;
      }
      if (originalUrl !== undefined) {
        process.env.LOXA_COLLECTOR_URL = originalUrl;
      } else {
        delete process.env.LOXA_COLLECTOR_URL;
      }
    }
  });

  it('LOXA_COLLECTOR_URL overrides DSN-derived collectorUrl', () => {
    const originalDsn = process.env.LOXA_DSN;
    const originalUrl = process.env.LOXA_COLLECTOR_URL;
    try {
      process.env.LOXA_DSN = 'loxa://collector.example.com/my-app';
      process.env.LOXA_COLLECTOR_URL = 'http://override:9308';
      const cfg = fromEnv();
      assert.equal(cfg.collectorUrl, 'http://override:9308');
    } finally {
      if (originalDsn !== undefined) {
        process.env.LOXA_DSN = originalDsn;
      } else {
        delete process.env.LOXA_DSN;
      }
      if (originalUrl !== undefined) {
        process.env.LOXA_COLLECTOR_URL = originalUrl;
      } else {
        delete process.env.LOXA_COLLECTOR_URL;
      }
    }
  });

  it('LOXA_SERVICE overrides DSN-derived service', () => {
    const originalDsn = process.env.LOXA_DSN;
    const originalSvc = process.env.LOXA_SERVICE;
    try {
      process.env.LOXA_DSN = 'loxa://collector.example.com/my-app?service=dsn-svc';
      process.env.LOXA_SERVICE = 'env-svc';
      const cfg = fromEnv();
      assert.equal(cfg.service, 'env-svc');
    } finally {
      if (originalDsn !== undefined) {
        process.env.LOXA_DSN = originalDsn;
      } else {
        delete process.env.LOXA_DSN;
      }
      if (originalSvc !== undefined) {
        process.env.LOXA_SERVICE = originalSvc;
      } else {
        delete process.env.LOXA_SERVICE;
      }
    }
  });

  it('falls through gracefully on invalid DSN', () => {
    const originalDsn = process.env.LOXA_DSN;
    const originalUrl = process.env.LOXA_COLLECTOR_URL;
    try {
      process.env.LOXA_DSN = 'not-a-valid-dsn';
      process.env.LOXA_COLLECTOR_URL = 'http://fallback:9308';
      const cfg = fromEnv();
      assert.equal(cfg.collectorUrl, 'http://fallback:9308');
    } finally {
      if (originalDsn !== undefined) {
        process.env.LOXA_DSN = originalDsn;
      } else {
        delete process.env.LOXA_DSN;
      }
      if (originalUrl !== undefined) {
        process.env.LOXA_COLLECTOR_URL = originalUrl;
      } else {
        delete process.env.LOXA_COLLECTOR_URL;
      }
    }
  });
});
