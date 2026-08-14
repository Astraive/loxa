import pytest

from loza.core.client import Logger as ClientLogger
from loza.core.config import Config
from loza.core.logger import Logger
from loza.sinks.httpbatch import HTTPBatchSink


@pytest.mark.parametrize("logger_type", [Logger, ClientLogger])
def test_collector_base_url_targets_ingest_route(logger_type):
    logger = logger_type(Config.production("checkout").with_collector_endpoint("https://collector.example/"))

    sink = next(sink for sink in logger._config.sinks if isinstance(sink, HTTPBatchSink))

    assert sink.endpoint == "https://collector.example/events"

@pytest.mark.parametrize(
    ("dsn", "username", "password", "collector"),
    [
        (
            "loza://private-user:private-secret@collector.example/private-collector",
            "private-user",
            "private-secret",
            "private-collector",
        ),
        (
            "loza://lx_pub_6DJvd3D0izOaQx3n5BhKqN:@collector.example/public-collector",
            "lx_pub_6DJvd3D0izOaQx3n5BhKqN",
            "",
            "public-collector",
        ),
    ],
)
def test_credentialed_dsn_installs_scoped_credential_free_endpoint(
    dsn, username, password, collector
):
    logger = ClientLogger(Config.production("checkout").with_collector_endpoint(dsn))

    sink = next(sink for sink in logger._config.sinks if isinstance(sink, HTTPBatchSink))

    assert logger._config.collector_name == collector
    assert sink.endpoint == f"https://collector.example:443/collectors/{collector}/events"
    assert sink.username == username
    assert sink.password == password
    assert username not in sink.endpoint


def test_api_key_precedes_public_basic_auth() -> None:
    sink = HTTPBatchSink(
        "https://collector.example/collectors/public-collector/events",
        api_key="api-key",
        username="lx_pub_6DJvd3D0izOaQx3n5BhKqN",
        password="",
    )

    assert sink._auth_headers() == {"Authorization": "Bearer api-key"}
