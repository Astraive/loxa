// Shared test vectors from loza/spec/dsn/test-cases.json (25 cases: 12 valid, 13 invalid).
type ExpectedDsn<'a> = (
    &'a str,
    &'a str,
    u16,
    &'a str,
    &'a str,
    &'a str,
    bool,
    &'a str,
    &'a str,
    Option<&'a str>,
    Option<&'a str>,
    Option<&'a str>,
    Option<&'a str>,
);

fn assert_valid(input: &str, expected: ExpectedDsn<'_>) {
    let (
        expect_scheme,
        expect_host,
        expect_port,
        expect_project,
        expect_env,
        expect_service,
        expect_tls,
        expect_transport,
        expect_base_url,
        expect_events_url,
        expect_batch_url,
        expect_otlp_url,
        expect_tail_ws_url,
    ) = expected;
    let dsn = loza::dsn::parse(input).unwrap_or_else(|_| panic!("expected valid DSN: {input}"));
    assert_eq!(dsn.scheme, expect_scheme, "scheme mismatch for {input}");
    assert_eq!(dsn.host, expect_host, "host mismatch for {input}");
    assert_eq!(dsn.port, expect_port, "port mismatch for {input}");
    assert_eq!(dsn.project, expect_project, "project mismatch for {input}");
    assert_eq!(
        dsn.collector_name, expect_project,
        "collector_name mismatch for {input}"
    );
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
        (
            "loza",
            "localhost",
            9308,
            "demo",
            "default",
            "",
            false,
            "http",
            "http://localhost:9308",
            Some("http://localhost:9308/collectors/demo/events"),
            Some("http://localhost:9308/collectors/demo/events/batch"),
            Some("http://localhost:9308/collectors/demo/otlp/logs"),
            Some("ws://localhost:9308/collectors/demo/tail"),
        ),
    );
}

#[test]
fn localhost_default_port_9308() {
    assert_valid(
        "loza://localhost/demo?tls=false",
        (
            "loza",
            "localhost",
            9308,
            "demo",
            "default",
            "",
            false,
            "http",
            "http://localhost:9308",
            Some("http://localhost:9308/collectors/demo/events"),
            Some("http://localhost:9308/collectors/demo/events/batch"),
            Some("http://localhost:9308/collectors/demo/otlp/logs"),
            Some("ws://localhost:9308/collectors/demo/tail"),
        ),
    );
}

#[test]
fn prod_default_tls_true() {
    assert_valid(
        "loza://collector.example.com/demo",
        (
            "loza",
            "collector.example.com",
            443,
            "demo",
            "default",
            "",
            true,
            "http",
            "https://collector.example.com:443",
            Some("https://collector.example.com:443/collectors/demo/events"),
            Some("https://collector.example.com:443/collectors/demo/events/batch"),
            Some("https://collector.example.com:443/collectors/demo/otlp/logs"),
            Some("wss://collector.example.com:443/collectors/demo/tail"),
        ),
    );
}

#[test]
fn custom_env_and_service() {
    assert_valid(
        "loza://collector.example.com/demo?env=prod&service=api",
        (
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
        ),
    );
}

#[test]
fn otlp_transport() {
    assert_valid(
        "loza://collector.example.com/demo?transport=otlp",
        (
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
            Some("https://collector.example.com:443/collectors/demo/otlp/logs"),
            None,
        ),
    );
}

#[test]
fn grpc_transport() {
    assert_valid(
        "loza://collector.example.com/demo?transport=grpc",
        (
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
        ),
    );
}

#[test]
fn loopback_127_defaults_tls_false() {
    assert_valid(
        "loza://127.0.0.1/demo",
        (
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
        ),
    );
}

#[test]
fn ipv6_loopback_defaults_tls_false() {
    assert_valid(
        "loza://[::1]/demo",
        (
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
        ),
    );
}

#[test]
fn tls_auto_keeps_localhost_default() {
    assert_valid(
        "loza://localhost/demo?tls=auto",
        (
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
        ),
    );
}

#[test]
fn tls_auto_keeps_remote_default() {
    assert_valid(
        "loza://collector.example.com/demo?tls=auto",
        (
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
        ),
    );
}

#[test]
fn explicit_tls_true_on_localhost() {
    assert_valid(
        "loza://localhost:8443/demo?tls=true",
        (
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
        ),
    );
}

#[test]
fn explicit_port_4318_with_otlp() {
    assert_valid(
        "loza://collector.example.com:4318/backend?env=staging&service=auth&transport=otlp",
        (
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
        ),
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
fn parse_userinfo_credentials() {
    let dsn = loza::dsn::parse("loza://user:pass@collector.example.com/demo")
        .expect("credentialed DSN should parse");
    assert_eq!(dsn.username.as_deref(), Some("user"));
    assert_eq!(dsn.password.as_deref(), Some("pass"));
    assert!(!dsn.base_url.contains("user"));
    assert!(!dsn.base_url.contains("pass"));
    assert!(!format!("{dsn:?}").contains("Some(\"pass\")"));
}

#[test]
fn public_credentials_route_and_redact_bearer_capability() {
    let capability = "lz_pub_6DJvd3D0izOaQx3n5BhKqN";
    let dsn = loza::dsn::parse(&format!(
        "loza://{capability}:@collector.example.com/public-collector"
    ))
    .expect("public DSN must parse");

    assert_eq!(dsn.username.as_deref(), Some(capability));
    assert_eq!(dsn.password.as_deref(), Some(""));
    assert_eq!(dsn.collector_name, "public-collector");
    assert_eq!(
        dsn.events_url,
        "https://collector.example.com:443/collectors/public-collector/events"
    );
    assert!(!format!("{dsn:?}").contains(capability));
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

#[test]
fn scoped_lql_query_url_is_canonical() {
    let dsn = loza::dsn::parse("loza://collector.example.com/demo").unwrap();
    assert_eq!(
        dsn.lql_url,
        "https://collector.example.com:443/collectors/demo/lql/query"
    );
    assert_eq!(dsn.lql_query_url, dsn.lql_url);
}
