import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  resolve: {
    alias: {
      'node:test': fileURLToPath(new URL('./tests/node-test-vitest-shim.ts', import.meta.url)),
    },
  },
  test: {
    globals: true,
    environment: 'node',
    include: ['tests/**/*.test.ts'],
    exclude: ['tests/e2e-collector.test.ts'],
    coverage: {
      provider: 'v8',
      include: ['src/**/*.ts'],
      exclude: ['src/**/*.d.ts', 'src/generated/**'],
      reporter: ['text', 'json', 'html', 'lcov'],
      thresholds: {
        statements: 95,
        functions: 95,
        branches: 95,
        lines: 95,
      },
    },
  },
});
