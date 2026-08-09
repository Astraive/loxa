import unittest
from pathlib import Path
import sys
from unittest.mock import patch

repo_root = Path(__file__).resolve().parents[2]
spec_dir = repo_root / 'spec'
sys.path.insert(0, str(spec_dir))

from services.queue_prototype import LocalQueue
from services.stager import build_queue, stage_events

class TestStager(unittest.TestCase):
    def setUp(self):
        self.spec_dir = spec_dir
        self.queue = LocalQueue(str(self.spec_dir / 'tmp_stager_queue'))
        for f in list(self.queue.path.glob('*.json')):
            try: f.unlink()
            except: pass
        self.staging = self.spec_dir / 'tmp_staging'
        if self.staging.exists():
            for p in self.staging.rglob('*'):
                try:
                    if p.is_file(): p.unlink()
                except: pass

    def test_produce_keeps_events_with_same_clock_value(self):
        event = {
            "event_id": "stg-same-clock",
            "timestamp": "2026-05-18T12:30:45Z",
            "service": "s",
            "schema_version": "v1",
            "event_version": "v1",
            "event": "same-clock",
        }
        with patch("services.queue_prototype.time.time", return_value=1_716_035_845.0):
            first = Path(self.queue.produce(event))
            second = Path(self.queue.produce(event))

        self.assertNotEqual(first, second)
        self.assertTrue(first.exists())
        self.assertTrue(second.exists())

    def test_stage_two_events(self):
        e1 = {'event_id': 'stg-1', 'timestamp': '2026-05-18T12:30:45Z', 'service': 's', 'schema_version': 'v1', 'event_version': 'v1', 'event': 'a'}
        e2 = {'event_id': 'stg-2', 'timestamp': '2026-05-18T12:30:46Z', 'service': 's', 'schema_version': 'v1', 'event_version': 'v1', 'event': 'b'}
        self.queue.produce(e1)
        self.queue.produce(e2)
        n = stage_events(self.queue, self.staging, batch_size=10, compress=True)
        self.assertEqual(n, 2)
        # check files
        parts = list(self.staging.glob('**/stg-1.json.gz'))
        self.assertTrue(len(parts) == 1)
        parts2 = list(self.staging.glob('**/stg-2.json.gz'))
        self.assertTrue(len(parts2) == 1)

    def test_build_queue_local_backend(self):
        queue = build_queue(
            "local",
            queue_dir=str(self.spec_dir / "tmp_stager_queue"),
            kafka_brokers="",
            kafka_topic="",
            kafka_group_id="",
            kafka_dlq_topic="",
            kafka_auto_offset_reset="earliest",
            kafka_poll_timeout_seconds=1.0,
            kafka_flush_timeout_seconds=10.0,
            retry_policy=None,
        )
        self.assertIsInstance(queue, LocalQueue)

    def test_build_queue_kafka_requires_configuration(self):
        with self.assertRaises(ValueError):
            build_queue(
                "kafka",
                queue_dir=str(self.spec_dir / "tmp_stager_queue"),
                kafka_brokers="",
                kafka_topic="",
                kafka_group_id="",
                kafka_dlq_topic="",
                kafka_auto_offset_reset="earliest",
                kafka_poll_timeout_seconds=1.0,
                kafka_flush_timeout_seconds=10.0,
                retry_policy=None,
            )

    def test_build_queue_kafka_backend(self):
        sentinel = object()
        with patch("services.stager.KafkaAdapter", return_value=sentinel) as adapter_cls:
            queue = build_queue(
                "kafka",
                queue_dir=str(self.spec_dir / "tmp_stager_queue"),
                kafka_brokers="kafka:9092",
                kafka_topic="events",
                kafka_group_id="stager",
                kafka_dlq_topic="events.dlq",
                kafka_auto_offset_reset="latest",
                kafka_poll_timeout_seconds=0.5,
                kafka_flush_timeout_seconds=12.0,
                retry_policy=None,
            )
        self.assertIs(queue, sentinel)
        adapter_cls.assert_called_once()

    def tearDown(self):
        for f in list(self.queue.path.glob('*.json')):
            try: f.unlink()
            except: pass
        if self.staging.exists():
            for p in self.staging.rglob('*'):
                try:
                    if p.is_file(): p.unlink()
                except: pass

if __name__ == '__main__':
    unittest.main()
