#[cfg(test)]
mod tests {
    use super::super::*;
    use serde_json::json;
    use std::io::{Read, Write};
    use std::net::TcpListener;
    use std::thread;
    use std::time::Duration;

    #[test]
    fn internal_core_helpers_cover_parsing_and_transitions() {
        assert_eq!(core::level::Level::parse("DEBUG").as_str(), "debug");
        assert_eq!(core::level::Level::parse("warn").as_str(), "warn");
        assert_eq!(core::level::Level::parse("error").as_str(), "error");
        assert_eq!(core::level::Level::parse("fatal").as_str(), "fatal");
        assert_eq!(core::level::Level::parse("unknown").as_str(), "info");
        assert_eq!(core::pipeline_order().len(), 8);
        assert!(core::canonical::is_canonical("service"));
        assert!(!core::canonical::is_canonical("custom"));
        assert_eq!(
            core::dotkey::split_dot_key(".user..name."),
            vec!["user", "name"]
        );
        assert_eq!(core::dotkey::snake_key("user.name"), "user_name");
        assert_eq!(core::dotkey::camel_key("user.first_name"), "userFirst_name");

        for (raw, expected) in [
            (
                "canonical_wins",
                core::duplicate_policy::DuplicatePolicy::CanonicalWins,
            ),
            (
                "user_wins",
                core::duplicate_policy::DuplicatePolicy::UserWins,
            ),
            (
                "first_wins",
                core::duplicate_policy::DuplicatePolicy::FirstWins,
            ),
            (
                "last_wins",
                core::duplicate_policy::DuplicatePolicy::LastWins,
            ),
            (
                "keep_both",
                core::duplicate_policy::DuplicatePolicy::KeepBoth,
            ),
            (
                "error_on_duplicate",
                core::duplicate_policy::DuplicatePolicy::ErrorOnDuplicate,
            ),
        ] {
            assert_eq!(
                core::duplicate_policy::DuplicatePolicy::parse(raw),
                expected
            );
        }
        assert_eq!(
            core::duplicate_policy::DuplicatePolicy::parse("other"),
            core::duplicate_policy::DuplicatePolicy::LastWins
        );
        use self::core::duplicate_policy::DuplicatePolicy::*;
        assert_eq!(
            core::duplicate::resolve_duplicate(json!("old"), json!("new"), CanonicalWins).unwrap(),
            json!("old")
        );
        assert_eq!(
            core::duplicate::resolve_duplicate(json!("old"), json!("new"), FirstWins).unwrap(),
            json!("old")
        );
        assert_eq!(
            core::duplicate::resolve_duplicate(json!("old"), json!("new"), UserWins).unwrap(),
            json!("new")
        );
        assert_eq!(
            core::duplicate::resolve_duplicate(json!("old"), json!("new"), LastWins).unwrap(),
            json!("new")
        );
        assert_eq!(
            core::duplicate::resolve_duplicate(json!("old"), json!("new"), KeepBoth).unwrap(),
            json!(["old", "new"])
        );
        assert_eq!(
            core::duplicate::resolve_duplicate(json!(["old"]), json!("new"), KeepBoth).unwrap(),
            json!(["old", "new"])
        );
        assert!(
            core::duplicate::resolve_duplicate(json!("old"), json!("new"), ErrorOnDuplicate)
                .is_err()
        );

        clock::freeze_at(1234);
        assert_eq!(clock::unix_millis(), 1234);
        clock::unfreeze();
        assert!(clock::unix_millis() > 0);
        core::uuidv7::set_id_generator(Box::new(|| "evt_test".to_string()));
        assert_eq!(core::uuidv7::new_event_id(), "evt_test");
        core::uuidv7::reset_id_generator();
        assert!(core::uuidv7::new_event_id().starts_with("evt_"));
    }

    #[test]
    fn internal_encoding_pool_retry_safe_and_environment_helpers_cover_edges() {
        assert_eq!(jsonenc::compact(&json!({"a": 1})).unwrap(), r#"{"a":1}"#);
        assert!(jsonenc::pretty(&json!({"a": 1})).unwrap().contains('\n'));
        assert_eq!(jsonenc::json_encoder(&json!(true)).unwrap(), "true");
        assert!(jsonenc::pretty_json_encoder(&json!([1, 2])).is_ok());
        assert_eq!(jsonenc::string::escape_control_chars("a\nb\tc"), "abc");
        assert_eq!(jsonenc::string::truncate_utf8("abc", 4), "abc");
        assert_eq!(jsonenc::string::truncate_utf8("aébc", 2), "a");
        assert_eq!(jsonenc::number::finite_f64(1.5), Some(1.5));
        assert_eq!(jsonenc::number::finite_f64(f64::NAN), None);
        assert_eq!(jsonenc::number::clamp_u64(u64::MAX as u128 + 1), u64::MAX);

        let mut pool = pool::StringPool::default();
        assert_eq!(pool.take(), "");
        pool.put("reused".to_string());
        assert_eq!(pool.take(), "");
        assert_eq!(pool.take(), "");

        let policy = retry::RetryPolicy::default();
        assert_eq!(policy.delay(0), Duration::ZERO);
        assert_eq!(policy.delay(1), Duration::from_millis(100));
        assert_eq!(policy.delay(20), policy.max_delay);

        assert_eq!(safe::recover_to_error(|| 7).unwrap(), 7);
        assert_eq!(
            safe::recover_to_error(|| panic!("boom")).unwrap_err(),
            "boom"
        );
        assert_eq!(
            safe::recover_to_error(|| std::panic::panic_any(42_u8)).unwrap_err(),
            "panic"
        );
        let security = safe::SecurityConfig::default();
        assert!(security.redact_by_default);
        assert!(!security.allow_pii);

        let key = "LOZA_INTERNAL_TEST_BOOL";
        std::env::remove_var(key);
        assert!(env::bool_var(key, true));
        std::env::set_var(key, "yes");
        assert!(env::bool_var(key, false));
        std::env::set_var(key, "off");
        assert!(!env::bool_var(key, true));
        std::env::set_var(key, "invalid");
        assert!(env::bool_var(key, true));
        assert_eq!(env::var(key).as_deref(), Some("invalid"));
        std::env::remove_var(key);
    }

    #[test]
    fn internal_queue_and_transport_cover_delivery_and_http_shapes() {
        let mut batcher = queue::ByteBatcher::with_limits(3, 2);
        assert!(batcher.push("ab".into()).is_none());
        assert_eq!(batcher.push("c".into()), None);
        assert_eq!(batcher.push("d".into()).unwrap(), vec!["ab", "c"]);
        assert_eq!(batcher.len(), 1);
        assert_eq!(batcher.drain(), vec!["d"]);
        assert!(batcher.is_empty());
        let mut bounded = queue::ByteBatcher::new(0);
        assert!(bounded.push("x".into()).is_none());
        assert_eq!(bounded.push("y".into()).unwrap(), vec!["x"]);

        let offline = queue::MemoryOfflineBuffer::new(1);
        assert!(offline.enqueue("first".into()));
        assert!(!offline.enqueue("second".into()));
        assert_eq!(offline.len(), 1);
        assert_eq!(offline.drain(), vec!["second"]);
        assert!(offline.is_empty());

        let dir = std::env::temp_dir().join(format!("loza-rs-queue-{}", std::process::id()));
        let disk = queue::DiskOfflineBuffer::new(&dir, 1);
        disk.enqueue("event").unwrap();
        assert_eq!(disk.drain().unwrap(), vec!["event"]);
        std::fs::remove_dir_all(&dir).ok();
        assert_eq!(queue::DeliveryStats::default().snapshot().enqueued, 0);

        let unsupported = transport::Transport::new()
            .send(&transport::HttpRequest::new("PATCH", "http://127.0.0.1:1"));
        assert_eq!(
            unsupported.unwrap_err().kind(),
            std::io::ErrorKind::InvalidInput
        );
        for method in ["GET", "POST", "PUT", "DELETE"] {
            let listener = TcpListener::bind("127.0.0.1:0").unwrap();
            let address = listener.local_addr().unwrap();
            let server = thread::spawn(move || {
                let (mut stream, _) = listener.accept().unwrap();
                let mut request = Vec::new();
                let mut chunk = [0_u8; 512];
                loop {
                    let count = stream.read(&mut chunk).unwrap();
                    if count == 0 {
                        break;
                    }
                    request.extend_from_slice(&chunk[..count]);
                    if let Some(header_end) = request.windows(4).position(|w| w == b"\r\n\r\n") {
                        let headers = String::from_utf8_lossy(&request[..header_end]);
                        let body_len = headers
                            .lines()
                            .find_map(|line| line.strip_prefix("Content-Length:"))
                            .and_then(|value| value.trim().parse::<usize>().ok())
                            .unwrap_or(0);
                        if request.len() >= header_end + 4 + body_len {
                            break;
                        }
                    }
                }
                stream
                    .write_all(
                        b"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nX-Request-Id: req\r\nX-Deduped: 1\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}",
                    )
                    .unwrap();
                stream.flush().unwrap();
            });
            let mut request = transport::HttpRequest::new(method, format!("http://{address}"))
                .with_header("X-Test", "yes");
            if method == "POST" || method == "PUT" {
                request = request.with_body("{}");
            }
            let response = transport::Transport::with_timeout_ms(1_000)
                .send(&request)
                .unwrap();
            assert_eq!(response.status_code, 200);
            assert_eq!(response.body, "{}");
            assert_eq!(response.headers["x-request-id"], "req");
            server.join().unwrap();
        }
    }
}
