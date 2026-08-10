#[test]
fn typed_attribute_helpers_work() {
    assert_eq!(loza::money(4999.0).key, "money");
    assert_eq!(loza::percent(87.5).key, "percent");
    assert_eq!(loza::bytes(2048).key, "bytes");
    assert_eq!(loza::http_status(200).key, "http.status_code");
    assert_eq!(loza::bucket("pro").key, "bucket");
    assert!(loza::masked("secret").sensitive);
    assert_eq!(loza::url("https://example.com").key, "url");
    assert_eq!(loza::email_hash("user@example.com").key, "email.hash");
    assert_eq!(loza::ip_hash("127.0.0.1").key, "ip.hash");
}
