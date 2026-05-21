import unittest
from pathlib import Path
import sys

repo_root = Path(__file__).resolve().parents[2]
spec_dir = repo_root / 'loxa-spec'
# ensure services package importable
sys.path.insert(0, str(spec_dir))

from services.collector_example import handle_event
from services.queue_prototype import LocalQueue
from services.dedupe_store import LocalDedupeStore

class TestCollectorDedupe(unittest.TestCase):
    def setUp(self):
        self.spec_dir = spec_dir
        self.queue = LocalQueue(str(self.spec_dir / 'tmp_test_queue'))
        # ensure empty
        for f in list(self.queue.path.glob('*.json')):
            try: f.unlink()
            except: pass
        self.store = LocalDedupeStore(str(self.spec_dir / '.dedupe_test_store'))

    def test_idempotency(self):
        event = {
            'event_id': 'evt-test-1',
            'schema_version': 'v1',
            'event_version': 'v1',
            'timestamp': '2026-05-18T12:30:45Z',
            'service': 'svc',
            'event': 'x',
        }
        resp1 = handle_event(event, self.queue, self.store)
        resp2 = handle_event(event, self.queue, self.store)
        self.assertEqual(resp1['status'], 'accepted')
        self.assertEqual(resp2['status'], 'duplicate')
        files = list(self.queue.path.glob('*.json'))
        self.assertEqual(len(files), 1)

    def tearDown(self):
        for f in list(self.queue.path.glob('*.json')):
            try: f.unlink()
            except: pass
        try:
            # remove shelve files (platform dependent)
            for p in Path(self.spec_dir).glob('.dedupe_test_store*'):
                try: p.unlink()
                except: pass
        except Exception:
            pass

if __name__ == '__main__':
    unittest.main()
