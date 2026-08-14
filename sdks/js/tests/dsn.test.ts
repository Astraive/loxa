import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { parse } from '../src/config/dsn.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));
const testCasesPath = join(__dirname, '..', '..', '..', 'spec', 'dsn', 'test-cases.json');
const testCases = JSON.parse(readFileSync(testCasesPath, 'utf-8'));

describe('loza:// DSN parser', () => {
  for (const tc of testCases.cases) {
    if (tc.valid) {
      it(`valid: ${tc.name}`, () => {
        const dsn = parse(tc.input);
        for (const [key, expected] of Object.entries(tc.expected)) {
          assert.equal(
            dsn[key as keyof typeof dsn],
            expected,
            `field "${key}": expected ${JSON.stringify(expected)}, got ${JSON.stringify(dsn[key as keyof typeof dsn])}`,
          );
        }
      });
    } else {
      it(`invalid: ${tc.name}`, () => {
        assert.throws(() => parse(tc.input), {
          message: /invalid Loza DSN/,
        });
      });
    }
  }
});

it('decodes credentialed userinfo without retaining it in resolved URLs', () => {
  const dsn = parse('loza://key%40id:secret%3Avalue@example.com/project');
  assert.equal(dsn.username, 'key@id');
  assert.equal(dsn.password, 'secret:value');
  assert.equal(dsn.baseURL, 'https://example.com:443');
  assert.equal(dsn.eventsURL, 'https://example.com:443/events');
  assert.equal(dsn.baseURL.includes('secret'), false);
});

it('rejects empty, malformed, and unescaped credential components', () => {
  for (const raw of [
    'loza://:secret@example.com/project',
    'loza://key:@example.com/project',
    'loza://key:secret%2@example.com/project',
    'loza://key:secret/part@example.com/project',
  ]) {
    assert.throws(() => parse(raw), /invalid Loza DSN/);
  }
});

it('redacts credentials from string and JSON representations', () => {
  const dsn = parse('loza://key-id:s%40cret%3Avalue@example.com/project');
  assert.equal(dsn.toString().includes('s@cret:value'), false);
  assert.equal(JSON.stringify(dsn).includes('s@cret:value'), false);
  assert.equal(JSON.stringify(dsn).includes('key-id'), false);
});
