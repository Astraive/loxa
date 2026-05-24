import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loxa from '../src/index.ts';

afterEach(() => {
  loxa.reset();
});

it('covers client creation, configuration, and aliasing', async () => {
  const sink = new loxa.MemorySink();
  loxa.configure(loxa.production('catalog').withSink(sink));

  const logger = loxa.createLoxa({ service: 'catalog-child', sink: new loxa.MemorySink() });
  assert.equal(logger.getConfig().service, 'catalog-child');

  const builder = loxa
    .test('cfg')
    .withRelease('2026.05.24')
    .withNamespace('payments');
  const built = builder.build();
  assert.equal(built.release, '2026.05.24');
  assert.equal(built.namespace, 'payments');

  const aliased = loxa.alias('audit');
  assert.equal(aliased.getConfig().service, 'catalog');
  assert.equal(aliased.getConfig().alias, 'audit');

  await logger.shutdown();
});
