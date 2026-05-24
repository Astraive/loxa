use loxa::Config;
use std::sync::{Mutex, OnceLock};

#[test]
fn env_overrides_file_defaults() {
    let _guard = env_lock().lock().expect("env lock");
    std::env::set_var("LOXA_SERVICE_VERSION", "9.9.9");
    std::env::set_var("LOXA_ENVIRONMENT", "staging");
    std::env::set_var("LOXA_REGION", "ap-south-1");
    std::env::set_var(
        "LOXA_COLLECTOR_ENDPOINT",
        "http://collector.example/events",
    );
    std::env::set_var("LOXA_ASYNC_ENABLED", "true");

    let cfg = Config::test("checkout");

    std::env::remove_var("LOXA_SERVICE_VERSION");
    std::env::remove_var("LOXA_ENVIRONMENT");
    std::env::remove_var("LOXA_REGION");
    std::env::remove_var("LOXA_COLLECTOR_ENDPOINT");
    std::env::remove_var("LOXA_ASYNC_ENABLED");

    assert_eq!(cfg.version, "9.9.9");
    assert_eq!(cfg.environment, "test");
    assert_eq!(cfg.region, "ap-south-1");
    assert_eq!(cfg.collector_endpoint, "http://collector.example/events");
    assert!(!cfg.async_enabled);
}

fn env_lock() -> &'static Mutex<()> {
    static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
    LOCK.get_or_init(|| Mutex::new(()))
}
