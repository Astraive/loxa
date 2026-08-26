import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import {
  collectorRouteURL, defaultConfig, disabled, validateConfig, withOptions,
  development, dev, production, test as testPreset, ConfigBuilder,
  WithService, WithAlias, WithVersion, WithEnvironment, WithSink, WithSampler,
  WithRedactor, WithSchema, WithEventSchema, WithAsync, WithCollectorEndpoint,
  WithDuplicatePolicy, WithBasicAuth, WithStatsHandler, WithDeploymentID,
  WithIncludeHost, WithPanicRecovery, WithApiKey, WithRelease, WithNamespace,
  WithOtelBridge, WithRetry, WithTimeout, WithQueueSize, WithLogger,
} from '../src/config/config.ts';
import { memorySink } from '../src/sinks/standard-sinks.ts';
import { sampleNone } from '../src/sampling/sampler.ts';
import { redact } from '../src/redaction/redactor.ts';
import { OTelSchema } from '../src/core/schema.ts';
import { Logger } from '../src/core/logger.ts';
import { Event } from '../src/core/event.ts';

const noopLogger = new Logger({ service: '' });
describe('Config boundaries', () => {
  it('builds presets and applies every fluent option', () => {
    assert.equal(collectorRouteURL('http://collector///', 'tenant one', '/events'), 'http://collector/collectors/tenant%20one/events');
    assert.equal(collectorRouteURL('http://collector/', '', '/events'), 'http://collector/events');
    assert.equal(disabled().sink, null);
    assert.equal(development('dev').environment, 'development');
    assert.equal(dev('dev').environment, 'development');
    assert.equal(production('prod').strict, true);
    assert.equal(production('prod').async.enabled, true);
    assert.equal(testPreset().service, 'test');

    const sink = memorySink();
    const builder = new ConfigBuilder(defaultConfig());
    const configured = builder
      .withService('svc').withAlias('alias').withVersion('1').withEnvironment('test')
      .withCollectorUrl('http://localhost:9308/events').withCollectorName('collector')
      .withApiKey(' key ').withBasicAuth('', '').withSink(sink).withSinks(sink)
      .withSampler(sampleNone()).withRedactor(redact('secret')).withSchema(new OTelSchema())
      .withLevel('debug').withStrict(true).withDuplicatePolicy('merge').withAsync(true)
      .withCollectorEndpoint('http://localhost:9308/events').withStatsHandler({})
      .withDeploymentID('dep').withIncludeHost(false).withPanicRecovery(true)
      .withBatchSize(2).withFlushInterval(20).withEnableCompression(false).withRelease('r')
      .withNamespace('ns').withOtelBridge(true).withRetry(1).withTimeout(20).withQueueSize(3)
      .withLogger(noopLogger);
    assert.equal(configured, builder);
    const disabledConfig = builder.disabled();
    const event = new Event({ event: 'test' }, 'svc', 'test');
    assert.equal(disabledConfig.sampler(event), false);
  });

  it('applies all ConfigOption helpers and preserves defaults for compatibility no-ops', () => {
    const sink = memorySink();
    const schema = new OTelSchema();
    let cfg = defaultConfig();
    const options = [
      WithService('svc'), WithAlias('alias'), WithVersion('1'), WithEnvironment('prod'),
      WithSink(sink), WithSampler(sampleNone()), WithRedactor(redact('token')), WithSchema(schema),
      WithEventSchema(schema), WithAsync(true), WithCollectorEndpoint('http://localhost:9308'),
      WithDuplicatePolicy('drop'), WithBasicAuth('lz_pub_capability', ''), WithStatsHandler({}),
      WithDeploymentID('dep'), WithIncludeHost(false), WithPanicRecovery(true), WithApiKey(' key '),
      WithRelease('release'), WithNamespace('ns'), WithOtelBridge(true), WithRetry(2),
      WithTimeout(100), WithQueueSize(4), WithLogger(noopLogger),
    ];
    for (const option of options) cfg = option(cfg);
    assert.equal(cfg.service, 'svc');
    assert.equal(cfg.alias, 'alias');
    assert.equal(cfg.apiKey, 'key');
    assert.equal(cfg.async.enabled, true);
    assert.equal(cfg.async.queueSize, 4);
    assert.equal(cfg.maxRetries, 2);
    assert.equal(cfg.timeoutMs, 100);
    assert.equal(cfg.includeHost, false);
  });

  it('rejects invalid credential and endpoint combinations', () => {
    const embedded = defaultConfig();
    embedded.collectorUrl = 'https://user:pass@example.com/events';
    assert.throws(() => validateConfig(embedded), /credentials must not be embedded/);
    const passwordOnly = defaultConfig();
    passwordOnly.collectorUrl = 'https://collector.example/events';
    passwordOnly.password = 'secret';
    assert.throws(() => validateConfig(passwordOnly), /password requires a username/);
    const usernameOnly = defaultConfig();
    usernameOnly.collectorUrl = 'https://collector.example/events';
    usernameOnly.username = 'private-user';
    assert.throws(() => validateConfig(usernameOnly), /require a password/);
    const plaintext = defaultConfig();
    plaintext.collectorUrl = 'http://collector.example/events';
    plaintext.username = 'user';
    plaintext.password = 'pass';
    assert.throws(() => validateConfig(plaintext), /require HTTPS/);
    const localhost = defaultConfig();
    localhost.collectorUrl = 'http://localhost/events';
    localhost.username = 'user';
    localhost.password = 'pass';
    assert.equal(validateConfig(localhost), localhost);
  });

  it('handles collector DSNs with public capabilities', () => {
    const cfg = withOptions(defaultConfig(), {
      collectorUrl: 'loza://lz_pub_capability:@localhost:9308/project?service=payments&env=staging',
    });
    assert.equal(cfg.collectorUrl, 'http://localhost:9308');
    assert.equal(cfg.collectorName, 'project');
    assert.equal(cfg.username, 'lz_pub_capability');
    assert.equal(cfg.password, '');
    assert.equal(cfg.service, 'payments');
    assert.equal(cfg.environment, 'staging');
  });
  it('applies every direct option and both OTEL bridge branches', () => {
    const base = defaultConfig();
    const cfg = withOptions(base, {
      service: 'svc', alias: 'alias', version: '1', environment: 'test', release: 'r', namespace: 'ns',
      collectorUrl: 'http://localhost:9308', collectorName: 'collector', apiKey: 'key',
      username: 'user', password: 'pass', sampler: sampleNone(), redactor: redact('secret'),
      schema: new OTelSchema(), level: 'debug', strict: true, async: true, asyncQueueSize: 4,
      workers: 2, batchSize: 3, flushIntervalMs: 4, enableCompression: false, duplicatePolicy: 'drop',
      includeHost: false,
    });
    assert.equal(cfg.service, 'svc');
    assert.equal(cfg.async.queueSize, 4);
    assert.equal(cfg.async.workers, 2);
    assert.equal(cfg.includeHost, false);
    assert.equal(WithOtelBridge(false)(base), base);
  });
});
