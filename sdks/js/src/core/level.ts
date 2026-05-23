/** Log levels matching the LOXA spec. */
export const LevelDebug = 0;
export const LevelInfo = 1;
export const LevelNotice = 2;
export const LevelWarn = 3;
export const LevelError = 4;
export const LevelFatal = 5;

export type Level = typeof LevelDebug | typeof LevelInfo | typeof LevelNotice | typeof LevelWarn | typeof LevelError | typeof LevelFatal;

const LEVEL_NAMES: Record<number, string> = {
  [LevelDebug]: 'debug',
  [LevelInfo]: 'info',
  [LevelNotice]: 'notice',
  [LevelWarn]: 'warn',
  [LevelError]: 'error',
  [LevelFatal]: 'fatal',
};

export function parseLevel(s: string): Level {
  const lower = s.toLowerCase();
  for (const [k, v] of Object.entries(LEVEL_NAMES)) {
    if (v === lower) return Number(k) as Level;
  }
  return LevelInfo;
}

export function levelName(l: Level): string {
  return LEVEL_NAMES[l] ?? 'info';
}
