#!/usr/bin/env python3
"""Parse vitest bench output into JSON."""
import json, re, sys

results = []
with open(sys.argv[1], 'rb') as f:
    for raw_line in f:
        line = raw_line.decode('latin-1')
        stripped = line.strip()
        if stripped and len(stripped) > 0 and ord(stripped[0]) == 0xb7:
            parts = re.split(r'\s{2,}', stripped[1:].strip())
            if len(parts) >= 5:
                name = parts[0].strip()
                try:
                    hz = float(parts[1].replace(',', ''))
                    mean_ms = float(parts[4])
                    mean_ns = mean_ms * 1_000_000
                    results.append({'name': name, 'ns_per_op': round(mean_ns, 2), 'hz': round(hz, 2), 'pass': True})
                except:
                    pass

suite = sys.argv[2] if len(sys.argv) > 2 else 'sdk-js'
timestamp = sys.argv[3] if len(sys.argv) > 3 else ''
out_file = sys.argv[4] if len(sys.argv) > 4 else None

output = {
    'suite': suite,
    'component': 'sdks/js/bench',
    'timestamp': timestamp,
    'results': results,
    'summary': {'total': len(results), 'passed': len(results), 'failed': 0}
}

if out_file:
    with open(out_file, 'w') as f:
        json.dump(output, f, indent=2)
else:
    print(json.dumps(output, indent=2))
