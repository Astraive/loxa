import { afterEach, it } from 'node:test';
import assert from 'node:assert/strict';
import * as loza from '../src/index.ts';

afterEach(() => {
  loza.reset();
});

it('covers client creation, configuration, and aliasing', async () => {
  const sink = new loza.MemorySink();
  loza.configure(loza.production('catalog').withSink(sink));

  const logger = loza.createLoza({ service: 'catalog-child', sink: new loza.MemorySink() });
  assert.equal(logger.getConfig().service, 'catalog-child');

  const builder = loza
    .test('cfg')
    .withRelease('2026.05.24')
    .withNamespace('payments');
  const built = builder.build();
  assert.equal(built.release, '2026.05.24');
  assert.equal(built.namespace, 'payments');

  const aliased = loza.alias('audit');
  assert.equal(aliased.getConfig().service, 'catalog');
  assert.equal(aliased.getConfig().alias, 'audit');

  await logger.shutdown();
});
