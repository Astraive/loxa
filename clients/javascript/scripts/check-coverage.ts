import { readFileSync } from 'node:fs';

const coverageFile = process.argv[2] || 'coverage/lcov.info';
const records = readFileSync(coverageFile, 'utf8')
  .split('end_of_record')
  .map(record => record.trim())
  .filter(Boolean);
const sourceRecords = records.filter(record => {
  const source = record.match(/^SF:(.+)$/m)?.[1].replaceAll('\\', '/');
  return source?.startsWith('src/') && source.endsWith('.ts');
});
if (sourceRecords.length === 0) {
  throw new Error(`No production source records found in ${coverageFile}`);
}

let linesFound = 0;
let linesHit = 0;
for (const record of sourceRecords) {
  const totals = [...record.matchAll(/^DA:\d+,(\d+)$/gm)].map(match => Number(match[1]));
  linesFound += totals.length;
  linesHit += totals.filter(count => count > 0).length;
}
const percentage = linesFound === 0 ? 0 : (linesHit / linesFound) * 100;
console.log(`Production line coverage: ${linesHit}/${linesFound} (${percentage.toFixed(2)}%)`);
if (percentage < 95) {
  throw new Error(`Production line coverage ${percentage.toFixed(2)}% is below the required 95%`);
}
