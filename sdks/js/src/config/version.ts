/**
 * Load SDK version from loxa-js.yaml metadata file.
 *
 * Falls back to a hardcoded default if the file cannot be found or parsed.
 */

import { existsSync, readFileSync } from 'node:fs';
import { resolve, join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const FALLBACK_VERSION = '0.2.5';

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
 * Read version from loxa-js.yaml, searching standard locations.
 * Returns FALLBACK_VERSION if file not found or parsing fails.
 */
function loadVersion(): string {
  try {
    const candidates = [
      join(getPackageRoot(), 'loxa-js.yaml'),
      join(process.cwd(), 'loxa-js.yaml'),
    ];

    for (const path of candidates) {
      if (existsSync(path)) {
        try {
          const content = readFileSync(path, 'utf-8');
          for (const line of content.split('\n')) {
            const trimmed = line.trim();
            if (trimmed.startsWith('version:')) {
              const value = trimmed.slice('version:'.length).trim().replace(/^["']|["']$/g, '');
              if (value) return value;
            }
          }
        } catch {
          // Read failed, try next candidate
        }
      }
    }
  } catch {
    // Browser/non-Node environment
  }

  return FALLBACK_VERSION;
}

/** SDK version loaded from loxa-js.yaml at module init time. */
export const SDK_VERSION: string = loadVersion();
