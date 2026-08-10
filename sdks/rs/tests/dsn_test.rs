/// Shared test vectors from loza/spec/dsn/test-cases.json (25 cases: 12 valid, 13 invalid).

fn assert_valid(
    input: &str,
    expect_scheme: &str,
    expect_host: &str,
    expect_port: u16,
    expect_project: &str,
    expect_env: &str,
    expect_service: &str,
    expect_tls: bool,
    expect_transport: &str,
    expect_base_url: &str,
    expect_events_url: Option<&str>,
    expect_batch_url: Option<&str>,
    expect_otlp_url: Option<&str>,
    expect_tail_ws_url: Option<&str>,
) {
    let dsn = loza::dsn::parse(input).expect(&format!("expected valid DSN: {input}"));
    assert_eq!(dsn.scheme, expect_scheme, "scheme mismatch for {input}");
    assert_eq!(dsn.host, expect_host, "host mismatch for {input}");
    assert_eq!(dsn.port, expect_port, "port mismatch for {input}");
    assert_eq!(dsn.project, expect_project, "project mismatch for {input}");
    assert_eq!(dsn.env, expect_env, "env mismatch for {input}");
    assert_eq!(dsn.service, expect_service, "service mismatch for {input}");
    assert_eq!(dsn.tls, expect_tls, "tls mismatch for {input}");
    assert_eq!(
        dsn.transport, expect_transport,
        "transport mismatch for {input}"
    );
    assert_eq!(
        dsn.base_url, expect_base_url,
        "base_url mismatch for {input}"
    );
    if let Some(ev) = expect_events_url {
        assert_eq!(dsn.events_url, ev, "events_url mismatch for {input}");
    }
    if let Some(b) = expect_batch_url {
        assert_eq!(dsn.batch_url, b, "batch_url mismatch for {input}");
    }
    if let Some(o) = expect_otlp_url {
        assert_eq!(dsn.otlp_url, o, "otlp_url mismatch for {input}");
    }
    if let Some(t) = expect_tail_ws_url {
        assert_eq!(dsn.tail_ws_url, t, "tail_ws_url mismatch for {input}");
    }
}

fn assert_invalid(input: &str) {
    let result = loza::dsn::parse(input);
    assert!(result.is_err(), "expected invalid DSN but got: {result:?}");
}

// ── Valid cases (12) ────────────────────────────────────────────────────────

#[test]
fn localhost_dev_with_explicit_port() {
    assert_valid(
        "loza://localhost:9308/demo?tls=false",
        "loza",
        "localhost",
        9308,
        "demo",
        "default",
        "",
        false,
        "http",
        "http://localhost:9308",
        Some("http://localhost:9308/events"),
        Some("http://localhost:9308/events/batch"),
        Some("http://localhost:9308/otlp/logs"),
        Some("ws://localhost:9308/tail"),
    );
}

#[test]
fn localhost_default_port_9308() {
    assert_valid(
        "loza://localhost/demo?tls=false",
        "loza",
        "localhost",
        9308,
        "demo",
        "default",
        "",
        false,
        "http",
        "http://localhost:9308",
        Some("http://localhost:9308/events"),
        Some("http://localhost:9308/events/batch"),
        Some("http://localhost:9308/otlp/logs"),
        Some("ws://localhost:9308/tail"),
    );
}

#[test]
fn prod_default_tls_true() {
    assert_valid(
        "loza://collector.example.com/demo",
        "loza",
        "collector.example.com",
        443,
        "demo",
        "default",
        "",
        true,
        "http",
        "https://collector.example.com:443",
        Some("https://collector.example.com:443/events"),
        Some("https://collector.example.com:443/events/batch"),
        Some("https://collector.example.com:443/otlp/logs"),
        Some("wss://collector.example.com:443/tail"),
    );
}

#[test]
fn custom_env_and_service() {
    assert_valid(
        "loza://collector.example.com/demo?env=prod&service=api",
        "loza",
        "collector.example.com",
        443,
        "demo",
        "prod",
        "api",
        true,
        "http",
        "https://collector.example.com:443",
        None,
        None,
        None,
        None,
    );
}

#[test]
fn otlp_transport() {
    assert_valid(
        "loza://collector.example.com/demo?transport=otlp",
        "loza",
        "collector.example.com",
        443,
        "demo",
        "default",
        "",
        true,
        "otlp",
        "https://collector.example.com:443",
        None,
        None,
        Some("https://collector.example.com:443/otlp/logs"),
        None,
    );
}

#[test]
fn grpc_transport() {
    assert_valid(
        "loza://collector.example.com/demo?transport=grpc",
        "loza",
        "collector.example.com",
        443,
        "demo",
        "default",
        "",
        true,
        "grpc",
        "https://collector.example.com:443",
        None,
        None,
        None,
        None,
    );
}

#[test]
fn loopback_127_defaults_tls_false() {
    assert_valid(
        "loza://127.0.0.1/demo",
        "loza",
        "127.0.0.1",
        9308,
        "demo",
        "default",
        "",
        false,
        "http",
        "http://127.0.0.1:9308",
        None,
        None,
        None,
        None,
    );
}

#[test]
fn ipv6_loopback_defaults_tls_false() {
    assert_valid(
        "loza://[::1]/demo",
        "loza",
        "::1",
        9308,
        "demo",
        "default",
        "",
        false,
        "http",
        "http://[::1]:9308",
        None,
        None,
        None,
        None,
    );
}

#[test]
fn tls_auto_keeps_localhost_default() {
    assert_valid(
        "loza://localhost/demo?tls=auto",
        "loza",
        "localhost",
        9308,
        "demo",
        "default",
        "",
        false,
        "http",
        "http://localhost:9308",
        None,
        None,
        None,
        None,
    );
}

#[test]
fn tls_auto_keeps_remote_default() {
    assert_valid(
        "loza://collector.example.com/demo?tls=auto",
        "loza",
        "collector.example.com",
        443,
        "demo",
        "default",
        "",
        true,
        "http",
        "https://collector.example.com:443",
        None,
        None,
        None,
        None,
    );
}

#[test]
fn explicit_tls_true_on_localhost() {
    assert_valid(
        "loza://localhost:8443/demo?tls=true",
        "loza",
        "localhost",
        8443,
        "demo",
        "default",
        "",
        true,
        "http",
        "https://localhost:8443",
        None,
        None,
        None,
        None,
    );
}

#[test]
fn explicit_port_4318_with_otlp() {
    assert_valid(
        "loza://collector.example.com:4318/backend?env=staging&service=auth&transport=otlp",
        "loza",
        "collector.example.com",
        4318,
        "backend",
        "staging",
        "auth",
        true,
        "otlp",
        "https://collector.example.com:4318",
        None,
        None,
        None,
        None,
    );
}

// ── Invalid cases (13) ──────────────────────────────────────────────────────

#[test]
fn reject_empty_string() {
    assert_invalid("");
}

#[test]
fn reject_wrong_scheme_https() {
    assert_invalid("https://collector.example.com/demo");
}

#[test]
fn reject_wrong_scheme_http() {
    assert_invalid("http://collector.example.com/demo");
}

#[test]
fn reject_no_host() {
    assert_invalid("loza://");
}

#[test]
fn reject_triple_slash_no_host() {
    assert_invalid("loza:///project");
}

#[test]
fn reject_no_project() {
    assert_invalid("loza://collector.example.com");
}

#[test]
fn reject_empty_project() {
    assert_invalid("loza://collector.example.com/");
}

#[test]
fn reject_userinfo_key() {
    assert_invalid("loza://key@collector.example.com/demo");
}

#[test]
fn reject_userinfo_with_password() {
    assert_invalid("loza://user:pass@collector.example.com/demo");
}

#[test]
fn reject_invalid_tls_value() {
    assert_invalid("loza://collector.example.com/demo?tls=maybe");
}

#[test]
fn reject_invalid_transport_value() {
    assert_invalid("loza://collector.example.com/demo?transport=random");
}

#[test]
fn reject_port_zero() {
    assert_invalid("loza://collector.example.com:0/demo");
}

#[test]
fn reject_port_above_65535() {
    assert_invalid("loza://collector.example.com:99999/demo");
}

#[test]
fn reject_non_numeric_port() {
    assert_invalid("loza://collector.example.com:abc/demo");
}
