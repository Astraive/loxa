#[test]
fn typed_attribute_helpers_work() {
    assert_eq!(loxa::money(4999.0).key, "money");
    assert_eq!(loxa::percent(87.5).key, "percent");
    assert_eq!(loxa::bytes(2048).key, "bytes");
    assert_eq!(loxa::http_status(200).key, "http.status_code");
    assert_eq!(loxa::bucket("pro").key, "bucket");
    assert!(loxa::masked("secret").sensitive);
    assert_eq!(loxa::url("https://example.com").key, "url");
    assert_eq!(loxa::email_hash("user@example.com").key, "email.hash");
    assert_eq!(loxa::ip_hash("127.0.0.1").key, "ip.hash");
}
