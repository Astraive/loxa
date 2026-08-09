import pytest

from loxa.core.client import Logger as ClientLogger
from loxa.core.config import Config
from loxa.core.logger import Logger
from loxa.sinks.httpbatch import HTTPBatchSink


@pytest.mark.parametrize("logger_type", [Logger, ClientLogger])
def test_collector_base_url_targets_ingest_route(logger_type):
    logger = logger_type(Config.production("checkout").with_collector_endpoint("https://collector.example/"))

    sink = next(sink for sink in logger._config.sinks if isinstance(sink, HTTPBatchSink))

    assert sink.endpoint == "https://collector.example/events"
