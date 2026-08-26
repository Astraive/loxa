use loza::Config;
use std::sync::{Mutex, OnceLock};

#[test]
fn env_overrides_file_defaults() {
    let _guard = env_lock().lock().expect("env lock");
    std::env::set_var("LOZA_SERVICE_VERSION", "9.9.9");
    std::env::set_var("LOZA_ENVIRONMENT", "staging");
    std::env::set_var("LOZA_REGION", "ap-south-1");
    std::env::set_var("LOZA_COLLECTOR_ENDPOINT", "http://collector.example/events");
    std::env::set_var("LOZA_ASYNC_ENABLED", "true");

    let cfg = Config::test("checkout");

    std::env::remove_var("LOZA_SERVICE_VERSION");
    std::env::remove_var("LOZA_ENVIRONMENT");
    std::env::remove_var("LOZA_REGION");
    std::env::remove_var("LOZA_COLLECTOR_ENDPOINT");
    std::env::remove_var("LOZA_ASYNC_ENABLED");

    assert_eq!(cfg.version, "9.9.9");
    assert_eq!(cfg.environment, "test");
    assert_eq!(cfg.region, "ap-south-1");
    assert_eq!(cfg.collector_endpoint, "http://collector.example/events");
    assert!(!cfg.async_enabled);
}

#[test]
fn plaintext_basic_auth_is_allowed_for_local_dsn_only() {
    let cfg = Config::test("checkout")
        .with_dsn("loza://localhost/project?tls=false")
        .with_basic_auth("dsn-user", "dsn-secret");
    assert!(
        cfg.validate().is_ok(),
        "local plaintext DSNs should remain usable"
    );
}

#[test]
fn remote_plaintext_basic_auth_is_rejected() {
    let cfg = Config::test("checkout")
        .with_dsn("loza://dsn-user:dsn-secret@collector.example/project?tls=false");
    assert!(
        cfg.validate().is_err(),
        "remote plaintext Basic-auth DSNs must be rejected"
    );
}

#[test]
fn credentialed_dsn_configures_scoped_endpoint_and_preserves_api_key_precedence() {
    let capability = "lz_pub_6DJvd3D0izOaQx3n5BhKqN";
    let logger = loza::New(
        Config::production("checkout")
            .with_dsn(format!(
                "loza://{capability}:@collector.example/public-collector?env=prod"
            ))
            .with_api_key("api-key"),
    );

    assert_eq!(
        logger.config().collector_endpoint,
        "https://collector.example:443"
    );
    assert_eq!(logger.config().collector_name, "public-collector");
    match logger.config().sinks.first() {
        Some(loza::SinkConfig::HttpBatch {
            endpoint,
            api_key,
            basic_username,
            basic_password,
            ..
        }) => {
            assert_eq!(
                endpoint,
                "https://collector.example:443/collectors/public-collector/events"
            );
            assert_eq!(api_key.as_deref(), Some("api-key"));
            assert_eq!(basic_username.as_deref(), Some(capability));
            assert_eq!(basic_password.as_deref(), Some(""));
        }
        sink => panic!("expected scoped HTTP batch sink, got {sink:?}"),
    }
}

fn env_lock() -> &'static Mutex<()> {
    static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
    LOCK.get_or_init(|| Mutex::new(()))
}
