import { randomBytes } from 'node:crypto';

/**
 * Generate a UUIDv7 string (monotonic, sortable by time).
 * Format: xxxxxxxx-xxxx-7xxx-yxxx-xxxxxxxxxxxx
 */
export function uuidv7(): string {
  const now = Date.now();
  const bytes = randomBytes(16);

  // 48-bit timestamp (ms since epoch) in first 6 bytes
  bytes[0] = (now / 2**40) & 0xff;
  bytes[1] = (now / 2**32) & 0xff;
  bytes[2] = (now / 2**24) & 0xff;
  bytes[3] = (now / 2**16) & 0xff;
  bytes[4] = (now / 2**8) & 0xff;
  bytes[5] = now & 0xff;

  // Version 7
  bytes[6] = (bytes[6] & 0x0f) | 0x70;
  // Variant 10
  bytes[8] = (bytes[8] & 0x3f) | 0x80;

  const hex = bytes.toString('hex');
  return [
    hex.slice(0, 8),
    hex.slice(8, 12),
    hex.slice(12, 16),
    hex.slice(16, 20),
    hex.slice(20, 32),
  ].join('-');
}
