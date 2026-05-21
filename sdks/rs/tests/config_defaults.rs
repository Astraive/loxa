use std::fs;
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

#[test]
fn dev_config_uses_defaults_then_user_yaml() {
    let tmp = unique_temp_dir();
    fs::create_dir_all(&tmp).expect("mkdir temp");
    let defaults = tmp.join("loxa-rs.defaults.yaml");
    let user = tmp.join(".loxa-rs.yaml");
    fs::write(
        &defaults,
        "service: defaults-service\nenvironment: development\nlevel: info\nmax_event_bytes: 111\n",
    )
    .expect("write defaults");
    fs::write(&user, "service: user-service\nlevel: debug\n").expect("write user");

    std::env::set_var("LOXA_RS_DEFAULTS", defaults.as_os_str());
    std::env::set_var("LOXA_RS_CONFIG", user.as_os_str());

    let cfg = loxa::Config::dev("");
    assert_eq!(cfg.service, "");
    assert_eq!(cfg.level, "debug");
    assert_eq!(cfg.max_event_bytes, 111);

    std::env::remove_var("LOXA_RS_DEFAULTS");
    std::env::remove_var("LOXA_RS_CONFIG");
    let _ = fs::remove_dir_all(&tmp);
}

fn unique_temp_dir() -> PathBuf {
    let suffix = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("time")
        .as_nanos();
    std::env::temp_dir().join(format!("loxa-rs-config-{suffix}"))
}
