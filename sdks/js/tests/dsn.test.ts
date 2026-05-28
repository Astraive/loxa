import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { parse } from '../src/config/dsn.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));
const testCasesPath = join(__dirname, '..', '..', '..', 'spec', 'dsn', 'test-cases.json');
const testCases = JSON.parse(readFileSync(testCasesPath, 'utf-8'));

describe('loxa:// DSN parser', () => {
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
          message: /invalid Loxa DSN/,
        });
      });
    }
  }
});
