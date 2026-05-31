import { spawnSync } from 'node:child_process';
import { readdirSync } from 'node:fs';
import { join } from 'node:path';

const testsDir = new URL('../tests/', import.meta.url);
const testFiles = readdirSync(testsDir)
  .filter((file) => file.endsWith('.test.ts'))
  .sort()
  .map((file) => join('tests', file));

const result = spawnSync(process.execPath, [
  '--import',
  'data:text/javascript,import { register } from "node:module"; import { pathToFileURL } from "node:url"; register("./scripts/ts-loader.mjs", pathToFileURL("./"));',
  '--test',
  ...testFiles,
], {
  stdio: 'inherit',
});

process.exit(result.status ?? 1);
