import json
import sys
import unittest
from pathlib import Path

repo_root = Path(__file__).resolve().parents[2]
spec_dir = repo_root / "loxa-spec"
sys.path.insert(0, str(spec_dir))

from services.kafka_adapter import KafkaAdapter, KafkaConfig, RetryPolicy


class FakeMessage:
    def __init__(self, value, topic="events", partition=1, offset=9, key=None, error=None):
        self._value = value
        self._topic = topic
        self._partition = partition
        self._offset = offset
        self._key = key
        self._error = error

    def topic(self):
        return self._topic

    def partition(self):
        return self._partition

    def offset(self):
        return self._offset

    def key(self):
        return self._key

    def value(self):
        return self._value

    def error(self):
        return self._error


class FakeProducer:
    def __init__(self, _config, fail_first=0):
        self.calls = []
        self.fail_first = fail_first
        self.produce_attempts = 0
        self.flush_calls = []

    def produce(self, topic, value=None, key=None, headers=None, on_delivery=None):
        self.produce_attempts += 1
        if self.produce_attempts <= self.fail_first:
            raise RuntimeError("transient-produce-error")
        self.calls.append({"topic": topic, "value": value, "key": key, "headers": headers})
        if on_delivery is not None:
            on_delivery(None, object())

    def poll(self, _timeout):
        return None

    def flush(self, _timeout):
        self.flush_calls.append(_timeout)
        return 0


class FakeConsumer:
    def __init__(self, _config, messages=None):
        self.messages = list(messages or [])
        self.subscriptions = []
        self.commits = []
        self.closed = False

    def subscribe(self, topics):
        self.subscriptions = list(topics)

    def poll(self, _timeout):
        if not self.messages:
            return None
        return self.messages.pop(0)

    def commit(self, message=None, asynchronous=False):
        self.commits.append({"message": message, "asynchronous": asynchronous})

    def close(self):
        self.closed = True


class TestKafkaAdapter(unittest.TestCase):
    def test_produce_retries_with_backoff(self):
        producer = FakeProducer({}, fail_first=1)
        consumer = FakeConsumer({})
        sleeps = []

        adapter = KafkaAdapter(
            KafkaConfig(brokers="localhost:9092", topic="events", group_id="g1", dlq_topic="events.dlq"),
            retry_policy=RetryPolicy(max_attempts=3, base_delay_seconds=0.01, max_delay_seconds=0.1, jitter_ratio=0.0),
            producer_cls=lambda _cfg: producer,
            consumer_cls=lambda _cfg: consumer,
            sleep_fn=sleeps.append,
            random_fn=lambda: 0.0,
        )

        adapter.produce({"event_id": "evt-1"})

        self.assertEqual(producer.produce_attempts, 2)
        self.assertEqual(len(producer.calls), 1)
        self.assertEqual(producer.calls[0]["topic"], "events")
        self.assertEqual(sleeps, [0.01])

    def test_consume_ack_and_dlq_payload(self):
        valid_message = FakeMessage(json.dumps({"event_id": "evt-ok"}).encode("utf-8"), topic="events", offset=10)
        invalid_message = FakeMessage(b"{bad-json", topic="events", offset=11)

        producer = FakeProducer({})
        consumer = FakeConsumer({}, messages=[invalid_message, valid_message])

        adapter = KafkaAdapter(
            KafkaConfig(brokers="localhost:9092", topic="events", group_id="g1", dlq_topic="events.dlq"),
            producer_cls=lambda _cfg: producer,
            consumer_cls=lambda _cfg: consumer,
        )

        records = list(adapter.consume(batch_size=1, timeout=0))
        self.assertEqual(len(records), 1)
        source, payload = records[0]
        self.assertEqual(payload["event_id"], "evt-ok")
        self.assertEqual(source.offset, 10)

        adapter.ack(source)
        self.assertEqual(len(consumer.commits), 2)
        self.assertEqual(consumer.commits[0]["message"].offset(), 11)  # invalid JSON was DLQ'd and committed
        self.assertEqual(consumer.commits[1]["message"].offset(), 10)

        self.assertEqual(len(producer.calls), 1)
        self.assertEqual(producer.calls[0]["topic"], "events.dlq")
        dlq_payload = json.loads(producer.calls[0]["value"].decode("utf-8"))
        self.assertEqual(dlq_payload["attempts"], 1)
        self.assertIn("invalid-json", dlq_payload["reason"])

    def test_close_flushes_producer_and_consumer(self):
        producer = FakeProducer({})
        consumer = FakeConsumer({})

        adapter = KafkaAdapter(
            KafkaConfig(brokers="localhost:9092", topic="events", group_id="g1", dlq_topic="events.dlq"),
            producer_cls=lambda _cfg: producer,
            consumer_cls=lambda _cfg: consumer,
        )

        adapter.close()

        self.assertEqual(producer.flush_calls, [10.0])
        self.assertTrue(consumer.closed)


if __name__ == "__main__":
    unittest.main()
