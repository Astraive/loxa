import sys
import unittest
from pathlib import Path
from unittest.mock import patch

repo_root = Path(__file__).resolve().parents[2]
spec_dir = repo_root / "loxa-spec"
sys.path.insert(0, str(spec_dir))

from services.kafka_adapter import RetryPolicy
from services.stager import stage_events


class InMemoryQueue:
    def __init__(self, messages):
        self.messages = list(messages)
        self.acked = []
        self.dlq = []

    def consume(self, batch_size=10):
        for item in self.messages[:batch_size]:
            yield item

    def ack(self, source):
        self.acked.append(source)

    def send_to_dlq(self, payload, *, reason, attempts, source=None):
        self.dlq.append(
            {
                "payload": payload,
                "reason": reason,
                "attempts": attempts,
                "source": source,
            }
        )


class TestStagerResilience(unittest.TestCase):
    def test_retry_then_success(self):
        queue = InMemoryQueue(
            [("h-1", {"event_id": "evt-1", "timestamp": "2026-05-18T12:30:45Z", "event": "ok"})]
        )
        sleep_calls = []
        policy = RetryPolicy(max_attempts=3, base_delay_seconds=0.01, max_delay_seconds=0.1, jitter_ratio=0.0)

        with patch(
            "services.stager._stage_payload",
            side_effect=[OSError("disk-1"), OSError("disk-2"), Path("done.json.gz")],
        ):
            staged = stage_events(
                queue,
                spec_dir / "tmp_staging_resilience",
                retry_policy=policy,
                sleep_fn=sleep_calls.append,
                random_fn=lambda: 0.0,
            )

        self.assertEqual(staged, 1)
        self.assertEqual(queue.acked, ["h-1"])
        self.assertEqual(len(queue.dlq), 0)
        self.assertEqual(sleep_calls, [0.01, 0.02])

    def test_retry_exhaustion_goes_to_dlq(self):
        queue = InMemoryQueue(
            [("h-2", {"event_id": "evt-2", "timestamp": "2026-05-18T12:30:45Z", "event": "bad"})]
        )
        policy = RetryPolicy(max_attempts=3, base_delay_seconds=0.01, max_delay_seconds=0.1, jitter_ratio=0.0)
        sleep_calls = []

        with patch("services.stager._stage_payload", side_effect=IOError("disk-full")):
            staged = stage_events(
                queue,
                spec_dir / "tmp_staging_resilience",
                retry_policy=policy,
                sleep_fn=sleep_calls.append,
                random_fn=lambda: 0.0,
            )

        self.assertEqual(staged, 0)
        self.assertEqual(queue.acked, ["h-2"])
        self.assertEqual(len(queue.dlq), 1)
        self.assertEqual(queue.dlq[0]["attempts"], 3)
        self.assertIn("stage-failed", queue.dlq[0]["reason"])
        self.assertEqual(sleep_calls, [0.01, 0.02])


if __name__ == "__main__":
    unittest.main()
