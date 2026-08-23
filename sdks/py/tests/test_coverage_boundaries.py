from __future__ import annotations

import base64
import io
import json
from datetime import timedelta
from pathlib import Path
from types import SimpleNamespace
from urllib.error import HTTPError, URLError

import pytest

import loza
from loza.core import config as config_module
from loza.core import event as event_module
from loza.core import lql, metric, pipeline, redactor, sampler, schema
from loza.cortex import client as cortex_client
from loza.cortex import engine as cortex_engine
from loza.sinks import FileSink, MultiSink, NoopSink, StdoutSink
from loza.sinks.httpbatch import HTTPBatchSink
import loza.sinks.httpbatch.httpbatch as httpbatch_module


class _Response:
    def __init__(self, payload: bytes, status: int = 200, headers: dict[str, str] | None = None) -> None:
        self.payload = payload
        self.status = status
    def __iter__(self):
        return iter(self.payload.splitlines(keepends=True))

    def __enter__(self) -> _Response:
        return self

    def __exit__(self, *_args: object) -> None:
        return None

    def read(self, *_args: object) -> bytes:
        return self.payload

    def getcode(self) -> int:
        return self.status


class _RecordingSink:
    def __init__(self) -> None:
        self.events: list[str] = []
        self.batches: list[list[str]] = []
        self.flushed = False
        self.closed = False

    def write(self, encoded: str) -> None:
        self.events.append(encoded)

    def write_batch(self, encoded_events: list[str]) -> None:
        self.batches.append(list(encoded_events))

    def flush(self) -> None:
        self.flushed = True

    def close(self) -> None:
        self.closed = True


class _FlakySink:
    def __init__(self, failures: int) -> None:
        self.failures = failures
        self.calls = 0

    def write(self, _encoded: str) -> None:
        self.calls += 1
        if self.calls <= self.failures:
            raise OSError("transient")


def _event(**kwargs: object) -> loza.EventContext:
    return loza.EventContext(service="checkout", params=loza.Params(event="test.event", **kwargs))


def test_config_validation_and_layer_precedence(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    assert config_module._collector_ingest_endpoint(" https://collector/ ") == "https://collector/events"
    assert config_module._collector_ingest_endpoint("https://collector/events") == "https://collector/events"
    assert config_module._collector_ingest_endpoint("https://collector", "tenant/a") == (
        "https://collector/collectors/tenant%2Fa/events"
    )
    assert config_module._parse_simple_yaml("# comment\nservice: checkout\nasync_config:\n  enabled: true\n  queue_size: 9\n") == {
        "service": "checkout",
        "async_config": {"enabled": True, "queue_size": 9},
    }
    assert config_module._parse_simple_yaml("ignored\nvalue: 'quoted'\nnumber: 2") == {
        "value": "quoted",
        "number": 2,
    }
    assert config_module._merge_dicts({"nested": {"a": 1}, "x": 1}, {"nested": {"b": 2}, "x": 3}) == {
        "nested": {"a": 1, "b": 2},
        "x": 3,
    }

    loza.Config.test("service").validate()
    with pytest.raises(ValueError, match="unsupported level"):
        loza.Config(level="trace").validate()
    with pytest.raises(ValueError, match="queue_size"):
        loza.Config(async_config=loza.AsyncConfig(queue_size=0)).validate()
    with pytest.raises(ValueError, match="workers"):
        loza.Config(async_config=loza.AsyncConfig(workers=0)).validate()
    with pytest.raises(ValueError, match="max_event_bytes"):
        loza.Config(security=loza.SecurityConfig(max_event_bytes=0)).validate()
    with pytest.raises(ValueError, match="strict mode"):
        loza.Config(strict=True).validate()
    with pytest.raises(ValueError, match="password requires"):
        loza.Config(password="secret").validate()
    with pytest.raises(ValueError, match="requires a password"):
        loza.Config(username="basic").validate()
    with pytest.raises(ValueError, match="HTTPS"):
        loza.Config(collector_endpoint="http://remote.example/events", username="basic", password="secret").validate()

    monkeypatch.setenv("LOZA_DSN", "loza://lx_pub_abc:ignored@collector.example/tenant?env=prod&service=api")
    monkeypatch.setenv("LOZA_SERVICE_VERSION", "v2")
    monkeypatch.setenv("LOZA_STRICT", "yes")
    monkeypatch.setenv("LOZA_ASYNC_ENABLED", "true")
    monkeypatch.setenv("LOZA_MAX_BUFFER_SIZE", "12")
    monkeypatch.setenv("LOZA_BATCH_SIZE", "4")
    monkeypatch.setenv("LOZA_MAX_EVENT_BYTES", "1024")
    env_cfg = config_module._apply_env_vars(loza.Config())
    assert env_cfg.collector_endpoint == "https://collector.example:443"
    assert env_cfg.collector_name == "tenant" and env_cfg.environment == "prod" and env_cfg.service == "api"
    assert env_cfg.version == "v2" and env_cfg.strict and env_cfg.async_config.enabled
    assert env_cfg.async_config.queue_size == 12 and env_cfg.async_config.batch_size == 4
    assert env_cfg.security.max_event_bytes == 1024

    monkeypatch.setenv("LOZA_COLLECTOR_URL", "https://url.example")
    monkeypatch.setenv("LOZA_COLLECTOR_ENDPOINT", "https://endpoint.example")
    assert config_module._apply_env_vars(loza.Config()).collector_endpoint == "https://url.example"
    monkeypatch.delenv("LOZA_COLLECTOR_URL")
    assert config_module._apply_env_vars(loza.Config()).collector_endpoint == "https://endpoint.example"

    defaults = config_module._config_from_mapping(
        {"service": "file", "environment": "production", "async_config": {"enabled": True, "queue_size": 2}}
    )
    merged = config_module._merge_file_config(loza.Config(), defaults)
    assert merged.service == "file" and merged.environment == "production"
    code = loza.Config(service="code", sinks=[NoopSink()], async_config=loza.AsyncConfig(queue_size=3))
    merged = config_module._merge_code_config(merged, code)
    assert merged.service == "code" and merged.async_config.queue_size == 3

    defaults_file = tmp_path / "defaults.yaml"
    defaults_file.write_text("service: from-file\n", encoding="utf-8")
    monkeypatch.setenv("LOZA_PY_DEFAULTS", str(defaults_file))
    assert config_module._find_defaults_path() == defaults_file
    user_file = tmp_path / "user.yaml"
    user_file.write_text("service: user\n", encoding="utf-8")
    monkeypatch.setenv("LOZA_PY_CONFIG", str(user_file))
    assert config_module._find_user_config_path() == user_file


def test_lql_queries_encode_values_and_decode_failures(monkeypatch: pytest.MonkeyPatch) -> None:
    result = lql.QueryResult(["id"], [{"id": 1}], duration_ms=2)
    assert len(result) == 1 and list(result) == [{"id": 1}] and "columns=1" in repr(result)
    assert lql.QueryValue("x").as_json() == {"value": "x"}
    assert lql.QueryValue(2, "int").as_json() == {"type": "int", "value": 2}

    captured: list[object] = []

    def success(req: object, timeout: float) -> _Response:
        captured.append((req, timeout))
        return _Response(json.dumps({"columns": [{"name": "id"}, "name"], "rows": [{"id": 1}], "row_count": 1}).encode())

    monkeypatch.setattr(lql.urllib.request, "urlopen", success)
    result = lql.query_lql(
        "https://collector/",
        "select",
        collector="tenant/a",
        parameters={"typed": lql.QueryValue(1, "int"), "raw": {"value": 2}, "plain": 3},
        limit=5000,
        api_key="key",
        env="prod",
        service="checkout",
    )
    req, timeout = captured[0]
    assert req.full_url.endswith("/collectors/tenant%2Fa/lql/query")
    body = json.loads(req.data)
    assert body["limit"] == 1000 and body["parameters"]["typed"]["type"] == "int"
    assert req.headers["Authorization"] == "Bearer key" and timeout == 30.0
    assert req.headers["X-loza-env"] == "prod" and req.headers["X-loza-service"] == "checkout"
    assert result.rows == [{"id": 1}]

    def basic(req: object, **_kwargs: object) -> _Response:
        assert base64.b64decode(req.headers["Authorization"].split()[1]) == b"u:p"
        return _Response(b'{"columns": [], "rows": []}')

    monkeypatch.setattr(lql.urllib.request, "urlopen", basic)
    assert lql.query_lql("https://collector", "select", username="u", password="p").row_count == 0

    error = HTTPError("https://collector", 422, "bad", {}, io.BytesIO(b'{"error":"invalid","diagnostics":[{"line":2}]}'))
    monkeypatch.setattr(lql.urllib.request, "urlopen", lambda *_args, **_kwargs: (_ for _ in ()).throw(error))
    with pytest.raises(lql.LQLCompilationError, match="invalid") as exc:
        lql.query_lql("https://collector", "bad")
    assert exc.value.status == 422 and exc.value.diagnostics

    bad_error = HTTPError("https://collector", 500, "bad", {}, io.BytesIO(b"not json"))
    monkeypatch.setattr(lql.urllib.request, "urlopen", lambda *_args, **_kwargs: (_ for _ in ()).throw(bad_error))
    with pytest.raises(lql.LQLCompilationError, match="HTTP 500"):
        lql.query_lql("https://collector", "bad")
    monkeypatch.setattr(lql.urllib.request, "urlopen", lambda *_args, **_kwargs: (_ for _ in ()).throw(OSError("down")))
    with pytest.raises(lql.LQLCompilationError, match="transport"):
        lql.query_lql("https://collector", "bad")

    monkeypatch.setattr(lql.urllib.request, "urlopen", lambda *_args, **_kwargs: _Response(b'{"columns": [], "rows": {}}'))
    with pytest.raises(lql.LQLCompilationError, match="invalid result"):
        lql.query_lql("https://collector", "bad")
    monkeypatch.setattr(lql.urllib.request, "urlopen", lambda *_args, **_kwargs: _Response(b'{"columns": [], "rows": []}'))
    assert lql.query_sql("https://collector", "select 1", api_key="sql-key").rows == []


def test_metrics_render_all_series_and_escaping() -> None:
    metrics = metric.MetricsCollector("checkout", buffer_capacity=-1, histogram_buckets=[1, 0.5, 1])
    metrics.on_event_created()
    metrics.on_event_finished()
    metrics.on_event_emitted(True)
    metrics.on_event_emitted(False)
    metrics.on_event_dropped('bad"\n')
    metrics.on_retry(0)
    metrics.on_backpressure()
    metrics.observe_emit_duration(timedelta(seconds=0.75))
    metrics.observe_emit_duration(-1)
    metrics.set_buffer_size(-2)
    metrics.set_buffer_capacity(4)
    snapshot = metrics.snapshot()
    assert snapshot["counters"]["events_created_total"] == 1
    assert snapshot["counter_vecs"]["retry_total"]["1"] == 1
    assert snapshot["gauges"] == {"buffer_size": 0.0, "buffer_capacity": 4.0}
    rendered = metrics.render_prometheus()
    assert 'checkout_events_dropped_total{reason="bad\\"\\n"} 1' in rendered
    assert "checkout_emit_duration_seconds_bucket" in rendered
    assert metric.RenderPrometheus(metrics) == rendered


def test_pipeline_batches_buffers_retries_and_shutdown(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    assert pipeline._safe_buffer_path(tmp_path / "buffer") == tmp_path / "buffer"
    with pytest.raises(ValueError):
        pipeline._safe_buffer_path("../unsafe")
    with pytest.raises(ValueError):
        pipeline._safe_buffer_path("bad\x00path")
    directory = tmp_path / "directory"
    directory.mkdir()
    with pytest.raises(ValueError):
        pipeline._safe_buffer_path(directory)

    stats = pipeline.DeliveryStats(enqueued=1)
    assert stats.snapshot()["enqueued"] == 1
    batcher = pipeline.ByteBatcher(max_batch_bytes=3, max_events=2)
    assert batcher.push("a") is None and batcher.pending == 1
    assert batcher.push("b") is None
    assert batcher.push("cc") == ["a", "b"]
    assert batcher.drain() == ["cc"] and batcher.pending == 0

    memory = pipeline.MemoryOfflineBuffer(2)
    memory.append("one")
    memory.append("two")
    memory.append("three")
    assert len(memory) == 2 and memory.dropped == 1 and memory.drain(1) == ["two"]
    assert memory.drain() == ["three"]

    disk = pipeline.DiskOfflineBuffer(tmp_path / "spool", max_bytes=20)
    disk.append("one")
    disk.append("two")
    disk.append("three")
    assert disk.drain(1)
    assert pipeline.RetryPolicy(base_delay=0.5, max_delay=0.75).delay(3) == 0.75

    sink = _RecordingSink()
    metrics = metric.MetricsCollector()
    pipe = pipeline.Pipeline([sink], queue_size=1, max_batch_bytes=10, metrics=metrics)
    assert pipe.try_enqueue('{"service":"checkout"}')
    assert not pipe.try_enqueue("overflow")
    assert pipe.drain_once() == 1 and sink.batches
    pipe.start(2)
    pipe.start(2)
    pipe.close(timeout=1)
    assert sink.flushed and sink.closed and pipe._closed

    monkeypatch.setattr(pipeline.time, "sleep", lambda _delay: None)
    flaky = _FlakySink(1)
    retry_pipe = pipeline.Pipeline([flaky], retry_policy=pipeline.RetryPolicy(max_attempts=2, base_delay=0))
    retry_pipe.write_sync("ok")
    assert flaky.calls == 2 and retry_pipe.stats.retried == 1
    errors: list[Exception] = []
    offline = pipeline.MemoryOfflineBuffer(5)
    failing = pipeline.Pipeline([_FlakySink(3)], retry_policy=pipeline.RetryPolicy(max_attempts=2, base_delay=0), offline_buffer=offline, error_handler=errors.append)
    failing.write_sync("lost")
    assert failing.stats.failed == 1 and failing.stats.emitted == 0 and len(offline) == 1 and errors
    encoded = json.loads(pipeline.encode_batch_envelope(['{"service":"api"}', "not-json", "{}"]))
    assert encoded["source"]["service"] == "api" and encoded["events"][1]["malformed"] == "not-json"


def test_redaction_sampling_and_schemas_cover_boundaries() -> None:
    payload = {"password": "secret", "nested": {"token": "abc", "keep": "x"}, "items": [{"password": "x"}]}
    assert redactor.redact_keys("password", "token")(payload)["password"] == redactor.REDACTED_VALUE
    assert payload["password"] == "secret"
    assert len(redactor.hash_keys("token")(payload)["nested"]["token"]) == 64
    assert "password" not in redactor.drop_keys("password")(payload)
    assert "*" in redactor.mask_keys("password")(payload)["password"]
    assert redactor.redact_patterns(r"secret")(payload)["password"] == redactor.REDACTED_VALUE
    assert redactor.sensitive_attrs_redactor(set())(payload) is payload
    assert "token" not in redactor.compose_redactors(redactor.redact_keys("password"), redactor.drop_keys("token"))(payload)["nested"]
    assert redactor.default_redactor()({"password": "secret"})["password"] == redactor.REDACTED_VALUE

    ctx = _event(method="GET", path="/checkout", route="checkout", status_code=500)
    ctx.user["id"] = "u1"
    ctx.tenant["id"] = "t1"
    ctx.attrs["feature"] = {"new": True}
    ctx.attrs["http.request.header.x-mode"] = "dark"
    ctx.outcome = "error"
    ctx.error = {"message": "bad"}
    ctx.started_at = ctx.started_at - timedelta(seconds=1)
    assert sampler.sample_all()(ctx) and not sampler.sample_none()(ctx)
    assert sampler.sample_random(-1)(ctx) is False and sampler.sample_random(2)(ctx) is True
    assert sampler.sample_errors()(ctx) and sampler.sample_slow_requests(timedelta(milliseconds=500))(ctx)
    assert sampler.sample_status_codes(500)(ctx) and sampler.sample_routes("checkout")(ctx)
    assert sampler.sample_users("u1")(ctx) and sampler.sample_tenants("t1")(ctx)
    assert sampler.sample_feature_flag("new", True)(ctx) and sampler.sample_by_header("X-Mode", "dark")(ctx)
    assert sampler.any_sampler(sampler.sample_none(), sampler.sample_all())(ctx)
    assert sampler.all_sampler(sampler.sample_all(), sampler.sample_errors())(ctx)
    assert sampler.not_sampler(sampler.sample_none())(ctx)
    assert sampler.sample_rate_limited(0)(ctx) is False
    limited = sampler.sample_rate_limited(1, 1)
    assert limited(ctx) is True and limited(ctx) is False
    assert sampler.sample_by_event(lambda event: event is ctx)(ctx)
    assert sampler.sample_by_outcome("error")(ctx) and sampler.should_sample(sampler.sample_all(), ctx)
    assert sampler.allow_fields("feature", "missing")(ctx) is False and sampler.block_fields("missing")(ctx)

    view = schema.EventView(ctx)
    assert view.id() == ctx.event_id and view.name() == "test.event" and view.service() == "checkout"
    assert view.kind() == "event" and view.level() == "info" and view.outcome() == "error"
    assert view.attr("feature.new") is True and view.attr("missing") is None
    assert view.group("user")["id"] == "u1" and view.group("missing") is None
    assert view.checkpoints() == [] and view.error() == {"message": "bad"}
    default = schema.DefaultSchema().encode(view)
    assert schema.NestedSchema().encode(view) == default
    assert "attrs_feature_new" in schema.FlatSchema().encode(view)
    assert schema.OTelLogSchema().encode(view)["body"] == "test.event"
    assert schema.ECSchema().encode(view)["service"]["name"] == "checkout"
    assert schema.DatadogSchema().encode(view)["ddsource"] == "loza"
    assert schema.CallableSchema(lambda event: {"id": event.id()}).encode(view) == {"id": ctx.event_id}


def test_event_helpers_and_sanitization_are_non_mutating() -> None:
    ctx = _event(trace_id="trace", span_id="span", incident_id="inc")
    ctx.params.version = "v1"
    ctx.params.environment = "prod"
    ctx.params.region = "us"
    ctx.params.deployment_id = "dep"
    ctx.params.message = "message"
    ctx.params.method = "GET"
    ctx.params.path = "/"
    ctx.params.route = "home"
    ctx.params.host = "host"
    ctx.params.status_code = 200
    ctx.user = {"id": "u"}
    ctx.tenant = {"id": "t"}
    ctx.resource = {"id": "r"}
    ctx.http = {"method": "GET"}
    ctx.attrs = {"nested": {"secret": "value"}}
    ctx.pii = {"email": "a@example.test"}
    ctx.checkpoints = [{"name": "start"}]
    ctx.processes = [{"name": "work"}]
    ctx.groups = [{"name": "group"}]
    ctx.timers = [{"name": "timer"}]
    ctx.partial = True
    ctx.partial_reason = "timeout"
    ctx.delivery_attempts = 2
    ctx.params.links = ["l1"]
    encoded = ctx.to_dict()
    assert encoded["http"]["status_code"] == 200 and encoded["links"] == ["l1"]
    assert ctx.id() == ctx.event_id and not ctx.is_finished() and not ctx.is_emitted()
    ctx._drop_keys.add("nested.secret")
    ctx._hash_keys.add("user.id")
    ctx._sensitive_keys.add("tenant.id")
    safe = event_module.sanitize_event(ctx)
    assert safe is not ctx and "secret" not in safe.attrs["nested"]
    assert safe.user["id"] != "u" and safe.tenant["id"] == "[REDACTED]" and ctx.user["id"] == "u"
    event_module._set_path_direct(ctx.attrs, "new.path", 1)
    assert event_module._get_path(ctx.attrs, "new.path") == 1
    event_module._delete_path(ctx.attrs, "new.path")
    assert event_module._get_path(ctx.attrs, "new.path") is None


def test_sinks_and_cortex_adapters(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    sink_a, sink_b = _RecordingSink(), _RecordingSink()
    fanout = MultiSink(sink_a, sink_b)
    fanout.write("one")
    fanout.write_batch(["two", "three"])
    fanout.flush()
    fanout.close()
    assert sink_a.events == ["one"] and sink_a.batches == [["two", "three"]] and sink_a.flushed and sink_a.closed

    path = tmp_path / "events.log"
    file_sink = FileSink(str(path))
    file_sink.write("event")
    file_sink.flush()
    file_sink.close()
    assert path.read_text(encoding="utf-8").splitlines() == ["event"]
    with pytest.raises(ValueError):
        FileSink(str(tmp_path / ".." / "bad"))
    output = io.StringIO()
    StdoutSink(output).write("stdout")
    assert output.getvalue().strip() == "stdout"
    NoopSink().write("ignored")

    responses = iter([_Response(b'{"status":"ok"}'), _Response(b'{"ready":true}'), _Response(b"metrics")])
    monkeypatch.setattr(cortex_client.urllib.request, "urlopen", lambda *_args, **_kwargs: next(responses))
    cortex = cortex_client.CortexClient("https://cortex", api_key="key")
    assert cortex.health() and cortex.ready() and cortex.metrics() == "metrics"
    assert cortex._headers()["Authorization"] == "Bearer key"
    assert cortex_client._clamp_positive_int("bad", 3, 10) == 3
    assert cortex_client._clamp_positive_int(-1, 3, 10) == 3 and cortex_client._clamp_positive_int(20, 3, 10) == 10

    responses = iter([_Response(b'{"incident_id":"i"}') for _ in range(16)])
    monkeypatch.setattr(cortex_client.urllib.request, "urlopen", lambda *_args, **_kwargs: next(responses))
    assert cortex.reconstruct("i").incident_id == "i"
    assert cortex.reconstruct_incident("i/x").incident_id == "i"
    assert cortex.service_graph("api", 0).nodes == [] and cortex.incident_graph("i", 101).edges == []
    cortex.record_remediation(loza.Remediation("i", "restart", "op"))
    cortex.record_feedback(loza.RemediationFeedback("r", "i", "success"))
    assert cortex.similar_incidents("i", 0) == []
    cortex.ingest_batch([])
    cortex.ingest_event({})
    monkeypatch.setattr(cortex_client.urllib.request, "urlopen", lambda *_args, **_kwargs: _Response(b""))
    cortex.ingest_jsonl([{"id": 1}])

    engine = cortex_engine.Engine("https://cortex")
    monkeypatch.setattr(cortex_engine.urllib.request, "urlopen", lambda *_args, **_kwargs: _Response(b"{}"))
    engine.ingest([{"id": 1}])
    engine.ingest([{"id": i} for i in range(256)])
    mapped = engine._map_response(
        {
            "incident_id": "i",
            "causal_chain": [{"event_id": "e", "timestamp": "t", "kind": "error", "service": "api", "attributes": {"cause_id": "c", "confidence": 0.9}, "description": "because"}],
            "similar_incidents": [{"incident_id": "old", "similarity": 0.8, "shape": "same"}],
            "suggested_actions": [{"action": "restart", "success_rate": 0.9, "avg_time_to_resolve_seconds": 2, "priority": 1}],
            "confidence": 0.7,
        }
    )
    assert mapped.related_events[0]["event_id"] == "e" and mapped.causal_chain[0].cause_id == "c"
    assert mapped.similar_past_incidents[0].past_incident_id == "old" and mapped.suggested_remediations[0].action == "restart"
    error = HTTPError("https://cortex", 500, "bad", {}, io.BytesIO(b"boom"))
    monkeypatch.setattr(cortex_engine.urllib.request, "urlopen", lambda *_args, **_kwargs: (_ for _ in ()).throw(error))
    with pytest.raises(RuntimeError, match="500"):
        engine._post("/bad", {})
    monkeypatch.setattr(cortex_engine.urllib.request, "urlopen", lambda *_args, **_kwargs: (_ for _ in ()).throw(URLError("down")))
    with pytest.raises(RuntimeError, match="connection"):
        engine._post("/bad", {})


def test_http_batch_auth_and_response_paths(monkeypatch: pytest.MonkeyPatch) -> None:
    calls: list[object] = []
    monkeypatch.setattr(
        httpbatch_module.urllib.request,
        "urlopen",
        lambda req, **kwargs: (calls.append((req, kwargs)) or _Response(b'{"acks":[{"event_id":"e"}],"errors":[],"request_id":"r"}')),
    )
    stats = SimpleNamespace(on_collector_ack=lambda **kwargs: calls.append(kwargs))
    sink = HTTPBatchSink("https://collector/events", api_key="key", stats_handler=stats, enable_compression=False)
    sink.write_batch(['{"timestamp":"2025-01-01T00:00:00Z","schema_version":"v1","event_version":"v1","event_id":"e","request_id":"r","service":"s","event":"x","kind":"event","level":"info"}'])
    assert sink.last_collector_response and calls[-1]["request_id"] == "r"
    assert calls[0][0].headers["Authorization"] == "Bearer key"
    basic = HTTPBatchSink("https://collector/events", username="u", password="p", enable_compression=False)
    assert basic._auth_headers()["Authorization"] == "Basic " + base64.b64encode(b"u:p").decode()
    custom = HTTPBatchSink("https://collector/events", api_key="key", auth_header="X-Key")
    assert custom._auth_headers() == {"X-Key": "key"}
    ndjson = HTTPBatchSink("https://collector/events", ndjson=True, enable_compression=False)
    monkeypatch.setattr(httpbatch_module.urllib.request, "urlopen", lambda *_args, **_kwargs: _Response(b"{}"))
    ndjson.write_batch(["one\n", "two"])
    with pytest.raises(ValueError):
        HTTPBatchSink("http://remote.example/events", username="u", password="p")
    monkeypatch.setattr(httpbatch_module.urllib.request, "urlopen", lambda *_args, **_kwargs: (_ for _ in ()).throw(URLError("down")))
    with pytest.raises(RuntimeError, match="collector send failed"):
        HTTPBatchSink("https://collector/events", retries=0).write(
            '{"timestamp":"2025-01-01T00:00:00Z","schema_version":"v1","event_version":"v1","event_id":"e","service":"s","event":"x","kind":"event"}'
        )


def test_root_facade_and_lifecycle_surface() -> None:
    sink = loza.MemorySink()
    logger = loza.configure(loza.Test("root").with_sink(sink))
    assert loza.default() is logger
    assert loza.dev("d").service == "d"
    assert loza.production("p").strict
    assert loza.test("t").environment == "test"

    parent = loza.start_event(loza.Params(event="parent"))
    child = loza.start_event_from(parent, loza.Params(event="child"))
    assert child.service == parent.service
    for starter, event_name in (
        (loza.start_http_event, "http.request"),
        (loza.start_job_event, "job.run"),
        (loza.start_queue_event, "queue.process"),
        (loza.start_cli_event, "cli.run"),
        (loza.start_cron_event, "cron.tick"),
    ):
        ctx = starter(loza.Params())
        loza.enrich(ctx, loza.String("field", "value"))
        loza.finish(ctx, "success")
        assert loza.emit(ctx)
        assert event_name
    loza.enrich(parent, loza.String("a", 1))
    loza.append(parent, b=2)
    loza.set(parent, c=3)
    loza.merge(parent, "resource", loza.String("name", "r"))
    loza.merge(parent, "custom", nested=1)
    assert loza.get(parent, "a") == 1 and loza.get_group(parent, "resource") == {"name": "r"}
    loza.delete(parent, "a")
    loza.checkpoint(parent, "checkpoint", loza.String("stage", "one"))
    process = loza.process(parent, "work", value=1)
    loza.finish_process(process, done=True)
    process2 = loza.start_process(parent, "failed")
    loza.finish_process_error(process2, RuntimeError("bad"))
    timer = loza.timer(parent, "timer")
    loza.stop_timer(timer)
    timer2 = loza.start_timer(parent, "timer2")
    loza.stop_timer(timer2, done=True)
    group = loza.start_group(parent, "group")
    loza.finish_group(group)
    assert loza.stopwatch().elapsed() >= timedelta(0)
    loza.finish(parent, "success")
    assert loza.emit_event(parent)

    for fn in (loza.debug, loza.info, loza.notice, loza.warn, loza.error, loza.fatal):
        assert fn("message", key="value")
    for fn in (loza.event, loza.track, loza.audit, loza.security, loza.breadcrumb):
        assert fn("surface", key="value")
    assert loza.metric("metric", 1.2)
    assert loza.count("count") and loza.gauge("gauge", 1.0) and loza.histogram("hist", 1.0)
    assert loza.float("f", 1.0).value == 1.0 and loza.json("j", {"a": 1}).value == {"a": 1}

    current = loza.start_event(loza.Params(event="lifecycle"))
    assert loza.current_event() is None and loza.bind_event(current) is logger
    assert loza.wrap(current, lambda value: value + 1, 1) == 2
    run_ctx = loza.start_event(loza.Params(event="run"))
    assert loza.run(run_ctx, lambda _ctx: None)
    assert loza.run_event(loza.Params(event="run-event"), lambda _ctx: None)
    error_ctx = loza.start_event(loza.Params(event="error-run"))
    assert loza.run(error_ctx, lambda _ctx: (_ for _ in ()).throw(RuntimeError("boom")))
    error_ctx2 = loza.start_event(loza.Params(event="error-run-event"))
    assert loza.run_event(error_ctx2.params, lambda _ctx: (_ for _ in ()).throw(RuntimeError("boom")))
    dropped = loza.start_event(loza.Params(event="dropped"))
    loza.drop(dropped)
    cancelled = loza.start_event(loza.Params(event="cancelled"))
    loza.cancel(cancelled)
    abandoned = loza.start_event(loza.Params(event="abandoned"))
    loza.abandon(abandoned)
    retried = loza.start_event(loza.Params(event="retried"))
    loza.retry(retried)
    partial = loza.start_event(loza.Params(event="partial"))
    loza.partial(partial, "timeout")
    clone = loza.clone_event(partial)
    loza.link_event(partial, clone)
    assert clone is not partial

    request_ctx = loza.from_request(
        {"method": "post", "path": "/checkout?id=1", "route": "/checkout", "headers": {"X-Request-Id": "req", "X-Trace-Id": "trace", "User-Agent": "agent", "referer": "/from?x=1"}},
        logger,
    )
    assert loza.get(request_ctx, "http.method") == "POST"
    loza.finish(request_ctx, "success")
    loza.emit(request_ctx)
    assert loza.http_request(SimpleNamespace(method="GET", path="/")).value["method"] == "GET"
    assert loza.http_response(SimpleNamespace(status_code=201)).value["status_code"] == 201
    assert loza.max_attr_length(4)["max_field_bytes"] == 4
    assert loza.max_attrs(2)["max_attr_count"] == 2
    assert loza.cardinality_policy({"mode": "drop"}) == {"mode": "drop"}
    assert loza.validate_event({"event": "bad"})[0] is False
    normalized = loza.normalize_event({"request_id": "r", "event": "r"})
    assert normalized["requestId"] == "r"

    for fn, args in (
        (loza.UserID, ("u",)), (loza.TenantID, ("t",)), (loza.WorkspaceID, ("w",)),
        (loza.OrganizationID, ("o",)), (loza.SessionID, ("s",)), (loza.RequestID, ("r",)),
        (loza.TraceID, ("tr",)), (loza.SpanID, ("sp",)), (loza.FeatureFlag, ("f", True)),
        (loza.FeatureFlagBool, ("f", True)), (loza.Experiment, ("e", "a")), (loza.OrderID, ("o",)),
        (loza.CartID, ("c",)), (loza.ProductID, ("p",)), (loza.CustomerID, ("c",)), (loza.Plan, ("p",)),
        (loza.Currency, ("USD",)), (loza.Amount, (1,)), (loza.Country, ("US",)), (loza.Device, ("d",)),
        (loza.Platform, ("web",)), (loza.AppVersion, ("v",)), (loza.ErrorType, ("Type",)),
        (loza.ErrorCode, ("E",)), (loza.ErrorMessage, ("m",)), (loza.ErrorStack, ("s",)), (loza.Retryable, (True,)),
    ):
        assert fn(*args).key
    assert loza.OTelSchema() and loza.CustomSchema(lambda event: {}).encode(None) == {}
    assert loza.SampleAll() and loza.SampleNone() and loza.SampleRandom(1) and loza.SampleRate(1)
    assert loza.Redact("password") and loza.SampleErrors() and loza.SampleSlowRequests(timedelta(seconds=1))
    assert loza.SampleStatusCodes(200) and loza.SampleRoutes("/") and loza.SampleUsers("u")
    assert loza.SampleTenants("t") and loza.SampleFeatureFlag("f", True) and loza.SampleByHeader("x", "y")
    assert loza.AnySampler(loza.SampleAll()) and loza.AllSampler(loza.SampleAll())
    assert loza.NotSampler(loza.SampleNone()) and loza.SampleRateLimited(1)
    assert loza.DefaultRedactor() and loza.RedactKeys("password") and loza.HashKeys("password")
    assert loza.MaskKeys("password") and loza.DropKeys("password") and loza.RedactPatterns("secret")
    metrics = loza.NewMetricsCollector()
    assert loza.RenderPrometheus(metrics).startswith("# HELP")
    loza.flush()
    loza.shutdown()


def test_config_options_security_and_legacy_helpers(tmp_path: Path) -> None:
    from loza.core import config_options, params, security, standard_sinks

    cfg = loza.Config()
    option_calls = (
        (config_options.WithService, "service"), (config_options.WithVersion, "v"),
        (config_options.WithEnvironment, "test"), (config_options.WithSink, NoopSink()),
        (config_options.WithSampler, loza.SampleAll()), (config_options.WithRedactor, loza.DefaultRedactor()),
        (config_options.WithMetrics, loza.NewMetricsCollector()), (config_options.WithSchema, loza.OTelSchema()),
        (config_options.WithEventSchema, loza.OTelSchema()), (config_options.WithAsync, True),
        (config_options.WithCollectorEndpoint, "https://collector"), (config_options.WithDuplicatePolicy, loza.LastWins),
        (config_options.WithStatsHandler, object()), (config_options.WithDeploymentID, "dep"),
        (config_options.WithIncludeHost, True), (config_options.WithPanicRecovery, True),
        (config_options.WithExitOnFatal, False), (config_options.WithRelease, "r"),
        (config_options.WithNamespace, "n"), (config_options.WithApiKey, "k"),
        (config_options.WithOtelBridge, True), (config_options.WithRetry, True),
        (config_options.WithTimeout, 1.0), (config_options.WithQueueSize, 4),
        (config_options.WithFlushInterval, 2), (config_options.WithBatchSize, 2),
    )
    for factory, value in option_calls:
        cfg = factory(value)(cfg)
    cfg = config_options.WithLogger("logger")(cfg)
    assert cfg.service == "service" and cfg.async_config.enabled and cfg.async_config.queue_size == 4
    assert cfg.async_config.flush_interval_ms == 2 and cfg.async_config.batch_size == 2
    assert config_options.Disabled().environment == "test" and config_options.FromEnv()

    limiter = security.SecurityLimiter(loza.SecurityConfig(max_event_bytes=5))
    assert not limiter.check_payload({"long": "value"}).allowed
    assert security.SecurityLimiter(loza.SecurityConfig(max_attr_count=1)).check_payload({"a": 1, "b": 2}).reason == "max_attr_count"
    assert security.SecurityLimiter(loza.SecurityConfig(max_field_bytes=1)).check_payload({"a": "xx"}).reason == "max_field_bytes"
    assert security.SecurityLimiter().check_payload({"a": 1}).allowed
    assert len(security.hash_value("secret")) == 64 and security.sensitive_string("secret", "x").sensitive
    assert security.hash_string("secret", "x").hash_value

    stderr = standard_sinks.StderrSink()
    stderr.write("ok")
    stderr.flush() if hasattr(stderr, "flush") else None
    rotating = standard_sinks.RotatingFileSink(str(tmp_path / "rotate.log"), max_bytes=2, max_backups=2)
    rotating.write("one")
    rotating.write("two")
    rotating.flush()
    rotating.close()
    assert (tmp_path / "rotate.log.1").exists()
    assert standard_sinks.CollectorSink().endpoint.endswith("/events")

    assert params.http_params("GET", "/", service="s").kind == "http"
    assert params.job_params("job").kind == "job"
    assert params.queue_params("q", "m").request_id == "m"
    assert params.cli_params("cmd").kind == "cli"
    assert params.cron_params("cron").kind == "cron"
    assert params.with_trace(params.job_params("j"), "trace", "span").trace_id == "trace"


def test_collector_client_helpers_and_http_paths(monkeypatch: pytest.MonkeyPatch) -> None:
    from loza.core import http_client

    valid = '{"timestamp":"2025-01-01T00:00:00Z","schema_version":"v1","event_version":"v1","event_id":"e","request_id":"r","service":"s","event":"x","kind":"event"}'
    assert isinstance(http_client._build_ingest_body([valid], "sdk", "v1", "s"), bytes)
    with pytest.raises(ValueError, match="at least one"):
        http_client._build_ingest_body([], "sdk", "v1", "s")
    with pytest.raises(ValueError, match="objects"):
        http_client._decode_event("[]")
    with pytest.raises(ValueError):
        http_client._parse_collector_response_body(b"[]")
    assert http_client._parse_collector_response_body(b"")[0] == {}
    response = SimpleNamespace(accepted=1, rejected=0, invalid=0, error="", reason="")
    assert http_client._collector_response_summary(response).startswith("accepted")
    assert http_client._retry_delay(2, 1, 2) == 2 and http_client._retry_delay(1, 1, 2, -1) == 0
    assert http_client._parse_retry_after("2") == 2 and http_client._parse_retry_after("") is None
    assert http_client._parse_retry_after("not-a-date") is None
    http_client._validate_ingest_envelope({"api_version": "v1", "source": {"sdk": "s", "version": "v", "service": "x"}, "events": [{}]})
    with pytest.raises(ValueError):
        http_client._validate_ingest_envelope({})
    assert http_client._private_endpoint_allowed() is False
    assert http_client._default_port("https") == 443
    assert http_client.WrapHTTPClient() and http_client.NewRoundTripper()
    client = http_client.CollectorClient("https://collector/events", api_key="key", retries=0)
    assert client._base_url() == "https://collector"
    assert client._auth_headers()["Authorization"] == "Bearer key"

    responses = iter([_Response(b'{"accepted":1,"rejected":0,"invalid":0}'), _Response(b'{"status":"ok"}'), _Response(b'{"status":"ready"}'), _Response(b'{"version":"v"}'), _Response(b'{"state":"ok"}')])
    monkeypatch.setattr(http_client, "urlopen", lambda *_args, **_kwargs: next(responses))
    assert client.send_batch([valid]).accepted == 1
    assert client.health() and client.ready() and client.version()["version"] == "v" and client.status()["state"] == "ok"

    lines_response = _Response(b'{"event":"one"}\nnot-json\n\n')
    monkeypatch.setattr(http_client, "urlopen", lambda *_args, **_kwargs: lines_response)
    assert list(client.tail_lines(service="s", kind="event")) == [{"event": "one"}, "not-json"]

    monkeypatch.setattr(http_client, "urlopen", lambda *_args, **_kwargs: _Response(b"{}"))
    assert client.validate({"event": "e"}) == {}
    assert client.query(q="x") == {} and client.tail(service="s") == {}
    assert client.replay(id="e") == {} and client.dlq_list() == {} and client.dlq_read("d") == {}
    assert client.dlq_replay("d") == {} and client.keys_create(name="k") == {}
    assert client.keys_revoke("k") == {} and client.keys_rotate("k") == {} and client.sinks_list() == {}
    assert client.sinks_test("sink") == {} and client.policy_validate({"x": 1}) == {}
    assert client.schema_check({"x": 1}) == {} and client.schema_publish({"x": 1}) == {}
    assert client.retention_apply() == {}
    with pytest.raises(ValueError, match="delete requires"):
        client.delete()
    assert client.delete(event_id="e") == {} and client.delete(tenant_id="t") == {} and client.delete(user_id="u") == {}


def test_legacy_logger_and_duplicate_policies() -> None:
    from loza.core.client import Logger as LegacyLogger

    sink = loza.MemorySink()
    cfg = loza.Config.test("legacy").with_sink(sink)
    legacy = LegacyLogger(cfg)
    ctx = legacy.start_event(loza.Params(event="legacy", user_id="u", tenant_id="t", workspace_id="w", custom=[loza.String("custom", 1)]))
    legacy.enrich(ctx, loza.String("user.name", "U"), loza.String("tenant.name", "T"), loza.String("resource.id", "R"), loza.String("http.path", "/"))
    legacy.append(ctx, plain=1)
    legacy.set(ctx, loza.String("set", 1))
    legacy.merge(ctx, "attrs", nested=1)
    legacy.merge(ctx, "unknown", value=1)
    legacy._legacy_enrich(ctx, old=1)
    legacy.delete(ctx, "old")
    ctx.checkpoint("checkpoint", stage="one")
    legacy.finish(ctx, "success")
    assert legacy.emit(ctx)
    legacy.flush()
    legacy.shutdown()
    assert sink.events
    for policy in (loza.CanonicalWins, loza.UserWins, loza.FirstWins, loza.LastWins, loza.KeepBoth):
        target: dict[str, object] = {"event": "canonical"}
        LegacyLogger._apply_duplicate(target, "event", "user", policy)
        assert target
    with pytest.raises(ValueError):
        LegacyLogger._apply_duplicate({"x": 1}, "x", 2, loza.ErrorOnDuplicate)
    target: dict[str, object] = {}
    LegacyLogger._set_path(target, "nested.path", 1)
    LegacyLogger._delete_path(target, "nested.path")
    legacy._enforce_attr_limit(ctx)
    assert legacy._target_map(ctx, "user") is ctx.user and legacy._target_map(ctx, "tenant") is ctx.tenant
    assert legacy._target_map(ctx, "resource") is ctx.resource and legacy._target_map(ctx, "http") is ctx.http
    assert legacy._target_map(ctx, "attrs") is ctx.attrs


def test_testkit_lifecycle_helpers_and_non_live_integrations() -> None:
    from loza.testkit import helpers as testkit
    from loza.integrations.logging.handler import LozaHandler
    from loza.integrations.loguru.adapter import LoguruSink
    from loza.integrations.structlog.adapter import StructlogProcessor, bind_loza, processor

    logger, sink = testkit.TestLogger("testkit")
    ctx = logger.start_event(loza.Params(event="test"))
    logger.finish(ctx, "success")
    encoded = logger.emit(ctx)
    payload = json.loads(encoded)
    assert payload["event"] == "test"
    events = testkit.DecodeEvents(sink)
    assert events and testkit.expect_event(events, event="test")
    testkit.AssertEvent(payload, event="test")
    testkit.expect_attr(payload, "event", "test")
    assert testkit.snapshot_event(payload)["event"] == "test"
    assert testkit.fake_clock()
    testkit.set_id_generator(lambda: "test_id")
    testkit.set_id_generator(None)
    assert testkit.reset_for_test() is None
    assert testkit.mock_sink()
    assert testkit.testkit()["logger"]
    assert testkit.events() == []
    assert testkit.last_event() is None
    testkit.clear_events()

    handler = LozaHandler()
    record = SimpleNamespace(
        name="logger",
        module="module",
        funcName="fn",
        lineno=1,
        getMessage=lambda: "message",
        levelno=20,
        exc_info=None,
        extra="value",
    )
    handler.emit(record)
    loguru = LoguruSink()
    loguru(SimpleNamespace(message="hello", record={"level": {"name": "INFO"}}))
    structlog = StructlogProcessor()
    event_dict = {"event": "info", "key": "value"}
    assert structlog(None, "info", event_dict)["key"] == "value"
    assert bind_loza(logger) is logger and processor(emit_immediate=False)


def test_active_logger_error_and_pipeline_lifecycle() -> None:
    from loza.core.logger import Logger

    class Stats:
        def __init__(self) -> None:
            self.events: list[object] = []
            self.drops: list[str] = []
            self.errors: list[Exception] = []

        def on_emit(self, event: object) -> None:
            self.events.append(event)

        def on_drop(self, reason: str) -> None:
            self.drops.append(reason)

        def on_error(self, error: Exception) -> None:
            self.errors.append(error)

        def on_collector_ack(self, **_kwargs: object) -> None:
            return None

        def on_delivery_failed(self, _event: object, _error: Exception) -> None:
            return None
    metrics = loza.NewMetricsCollector()
    sink = _RecordingSink()
    cfg = (
        loza.Config.test("active")
        .with_sink(sink)
        .with_metrics(metrics)
        .with_alias("alias")
        .with_version("v")
        .with_region("us")
    )
    cfg.stats_handler = None
    cfg.strict = False
    cfg.checkpoint_emit_immediately = False
    logger = Logger(cfg)
    ctx = logger.start_event(loza.Params(event="active", user_id="u", tenant_id="t", workspace_id="w"))
    logger.enrich(
        ctx,
        loza.Attr("drop", "x", drop=True),
        loza.Attr("hash", "x", hash_value=True),
        loza.Attr("sensitive", "x", sensitive=True),
        loza.Attr("user.name", "u"),
        loza.Attr("tenant.name", "t"),
        loza.Attr("resource.name", "r"),
        loza.Attr("http.path", "/"),
    )
    logger.merge(ctx, "attrs", nested={"x": 1})
    logger.merge(ctx, "unknown", value=1)
    logger._legacy_enrich(ctx, old=1)
    assert logger.get(ctx, "http.path") == "/"
    assert logger.get_group(ctx, "unknown") == {"value": 1}
    logger.delete(ctx, "old")
    logger.checkpoint(ctx, "checkpoint", phase="start")
    logger.finish(ctx, "success", final=True)
    encoded = logger.emit(ctx)
    assert encoded and sink.events
    assert logger.emit(ctx) == encoded
    logger.pause()
    logger.resume()
    assert logger.queue_size() == 0 and logger.health()
    logger.flush()
    logger.shutdown()

    sampled = Logger(loza.Config.test("sampled").with_sink(_RecordingSink()).with_sampler(lambda _event: False))
    sampled_ctx = sampled.start_event(loza.Params(event="sampled"))
    sampled.finish(sampled_ctx, "success")
    assert sampled.emit(sampled_ctx) == ""

    for policy in (loza.CanonicalWins, loza.FirstWins, loza.UserWins, loza.LastWins, loza.KeepBoth):
        policy_logger = Logger(loza.Config.test("policy").with_sink(_RecordingSink()).with_duplicate_policy(policy))
        policy_ctx = policy_logger.start_event(loza.Params(event="policy"))
        policy_logger.enrich(policy_ctx, loza.Attr("event", "user"))
        policy_logger.finish(policy_ctx, "success")
        policy_logger.emit(policy_ctx)
    strict_error = Logger(loza.Config.test("strict").with_sink(_RecordingSink()).with_duplicate_policy(loza.ErrorOnDuplicate))
    strict_ctx = strict_error.start_event(loza.Params(event="strict"))
    strict_error.enrich(strict_ctx, loza.Attr("event", "user"))
    strict_error.finish(strict_ctx, "success")
    with pytest.raises(loza.EventValidationError):
        strict_error.emit(strict_ctx)

    overflow_cfg = loza.Config.test("overflow").with_sink(_RecordingSink())
    overflow_cfg.security = loza.SecurityConfig(max_attr_count=1)
    overflow_logger = Logger(overflow_cfg)
    overflow_ctx = overflow_logger.start_event(loza.Params(event="overflow"))
    overflow_logger.enrich(overflow_ctx, a=1, b=2)
    overflow_logger.finish(overflow_ctx, "success")
    overflow_logger.emit(overflow_ctx)
    assert overflow_ctx.attrs.get("_truncated")

    pipeline_cfg = loza.Config.test("async").with_sink(_RecordingSink()).with_async(True)
    async_logger = Logger(pipeline_cfg)
    async_ctx = async_logger.start_event(loza.Params(event="async"))
    async_logger.finish(async_ctx, "success")
    async_logger.emit(async_ctx)
    async_logger.flush()
    async_logger.shutdown()

    bad_sink = _FlakySink(3)
    failing = Logger(loza.Config.test("failed").with_sink(bad_sink))
    failed_ctx = failing.start_event(loza.Params(event="failed"))
    failing.finish(failed_ctx, "success")
    assert failing.emit(failed_ctx) == "" and bad_sink.calls == 1
    assert not failed_ctx.emitted and failed_ctx.event_state == "delivery_failed"


def test_legacy_config_environment_and_attr_helpers(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    from datetime import datetime
    from loza.config import config as legacy_config
    from loza.env import env as legacy_env
    from loza.core import attr as attrs

    cfg = legacy_config.Config.dev("dev").with_service("svc").with_version("v").with_environment("test")
    cfg = cfg.with_region("us").with_sink(NoopSink()).with_sampler(lambda _: True).with_redactor(lambda x: x)
    cfg = cfg.with_metrics(loza.NewMetricsCollector()).with_schema(loza.DefaultSchema()).with_event_schema(loza.DefaultSchema())
    cfg = cfg.with_async(True).with_collector_endpoint("https://collector").with_duplicate_policy(legacy_config.LastWins)
    cfg.validate()
    for bad in (
        legacy_config.Config(service="x", level="bad"),
        legacy_config.Config(service="x", async_config=legacy_config.AsyncConfig(queue_size=0)),
        legacy_config.Config(service="x", async_config=legacy_config.AsyncConfig(workers=0)),
        legacy_config.Config(service="x", security=legacy_config.SecurityConfig(max_event_bytes=0)),
        legacy_config.Config(strict=True),
    ):
        with pytest.raises(ValueError):
            bad.validate()
    monkeypatch.setenv("LOZA_SERVICE", "env-service")
    monkeypatch.setenv("LOZA_STRICT", "yes")
    monkeypatch.setenv("LOZA_ASYNC_ENABLED", "true")
    monkeypatch.setenv("LOZA_MAX_BUFFER_SIZE", "4")
    monkeypatch.setenv("LOZA_BATCH_SIZE", "8")
    monkeypatch.setenv("LOZA_MAX_EVENT_BYTES", "16")
    applied = legacy_config._apply_env_vars(legacy_config.Config())
    assert applied.service == "env-service" and applied.strict and applied.async_config.queue_size == 4
    merged = legacy_config._merge_dicts({"nested": {"a": 1}}, {"nested": {"b": 2}})
    assert merged == {"nested": {"a": 1, "b": 2}}
    parsed = legacy_config._parse_simple_yaml("# comment\nservice: checkout\nasync_config:\n  enabled: true\n  queue_size: 2\nquoted: 'value'\nnumber: 2")
    assert parsed["service"] == "checkout" and parsed["async_config"]["enabled"] is True
    mapped = legacy_config._config_from_mapping(parsed)
    assert mapped.service == "checkout"
    assert legacy_config._find_defaults_path().name
    assert legacy_config._find_user_config_path() is None or isinstance(legacy_config._find_user_config_path(), Path)
    user = tmp_path / "config.yaml"
    user.write_text("service: user\n", encoding="utf-8")
    monkeypatch.setenv("LOZA_PY_CONFIG", str(user))
    assert legacy_config.load_layered_config().service == "user"
    monkeypatch.setenv("LOZA_SERVICE", "svc")
    assert legacy_config.new_client(legacy_config.Config(service="code"))

    monkeypatch.setenv("X_BOOL", "on")
    monkeypatch.setenv("X_INT", "bad")
    assert legacy_env.get("X_MISSING", "default") == "default"
    assert legacy_env.bool_env("X_BOOL") and legacy_env.bool_env("X_MISSING", True)
    assert legacy_env.int_env("X_INT", 3) == 3
    monkeypatch.setenv("LOZA_SERVICE", "svc")
    assert legacy_env.load_env_config().service == "svc"

    values = [
        attrs.String("s", "v"), attrs.Int("i", 1), attrs.Int64("i64", 1), attrs.Uint64("u", 1),
        attrs.Float64("f", 1.0), attrs.Bool("b", True), attrs.Time("t", datetime.now()), attrs.Duration("d", timedelta(seconds=1)),
        attrs.Any("a", object()), attrs.Null("n"), attrs.Group("g", attrs.String("nested.key", "v")),
        attrs.SensitiveString("ss", "v"), attrs.HashString("hs", "v"), attrs.MarkSensitive(attrs.String("m", "v")),
        attrs.list_("list", 1, 2), attrs.map_("map", {"x": 1}), attrs.enum_("enum", "x", "x"), attrs.id_("id", "x"),
        attrs.hash_("hash", "x"), attrs.redacted("red"), attrs.account_id("a"), attrs.deployment_id("d"),
        attrs.http_route("/"), attrs.http_method("get"), attrs.http_path("/"), attrs.http_user_agent("ua"),
        attrs.http_referer("https://x?a=1"), attrs.payment_id("p"), attrs.subscription_id("s"), attrs.invoice_id("i"),
        attrs.job_id("j"), attrs.message_id("m"), attrs.correlation_id("c"), attrs.commit_sha("sha"), attrs.release("r"),
        attrs.money("money", 1, "USD"), attrs.percent("pct", 1.0), attrs.bytes_attr("bytes", 1), attrs.http_status("status", 200),
        attrs.status_code("status", 200), attrs.error_code("err", "e"), attrs.bucket("bucket", "b"), attrs.tags("tags", "a", "b"),
        attrs.masked("mask", "123456", 2, 2), attrs.masked("short", "x"), attrs.url("https://x"), attrs.email_hash("E@x"), attrs.ip_hash("127.0.0.1"),
        attrs.region("us"), attrs.checkout_cart_item_count(1), attrs.checkout_cart_total(2), attrs.checkout_payment_method("card"),
        attrs.checkout_status("ok"), attrs.payment_provider("p"), attrs.payment_method("m"), attrs.payment_intent_id("i"),
        attrs.payment_failure_code("e"), attrs.payment_retry_attempt(1), attrs.billing_plan("p"), attrs.billing_subscription_id("s"),
        attrs.billing_invoice_id("i"), attrs.billing_amount(1), attrs.billing_interval("month"), attrs.agent_name("a"),
        attrs.agent_provider("p"), attrs.agent_model("m"), attrs.agent_run_type("r"), attrs.agent_tool_name("t"),
        attrs.agent_tool_outcome("o"), attrs.agent_input_tokens(1), attrs.agent_output_tokens(2), attrs.agent_cost(1.0),
        attrs.rag_index("i"), attrs.rag_embedding_model("m"), attrs.rag_chunks_retrieved(1), attrs.rag_top_score(1.0),
        attrs.rag_query_hash("q"), attrs.rag_citation_count(1), attrs.rag_retrieval_latency(1),
    ]
    assert all(item.key for item in values)
