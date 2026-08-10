from __future__ import annotations

import hashlib

import loza


def test_typed_attribute_helpers_snake_case():
    assert loza.money("price", 1000, "USD").key == "price"
    assert loza.percent("cpu", 85.5).key == "cpu"
    assert loza.bytes_attr("file.size", 1024).key == "file.size"
    assert loza.http_status("response.status", 200).key == "response.status"
    assert loza.status_code("code", 404).value == 404
    assert loza.error_code("err.code", "E123").value == "E123"
    assert loza.bucket("user.tier", "premium").key == "user.tier"
    assert loza.tags("env", "prod", "staging").key == "env"
    assert loza.masked("card", "4111111111111111").value == "41************11"
    assert loza.url("https://example.com").key == "url"
    assert loza.email_hash("User@Example.COM").value == hashlib.sha256(b"user@example.com").hexdigest()
    assert loza.ip_hash("192.168.1.1").value == hashlib.sha256(b"192.168.1.1").hexdigest()


def test_typed_attribute_helpers_pascal_case():
    assert loza.Money("price", 500, "EUR").key == "price"
    assert loza.Percent("mem", 72.3).key == "mem"
    assert loza.Bytes("disk", 2048).key == "disk"
    assert loza.HTTPStatus("status", 503).key == "status"
    assert loza.StatusCode("code", 404).value == 404
    assert loza.Bucket("env", "prod").key == "env"
    assert loza.Tags("region", "us", "eu").key == "region"
    assert loza.Masked("ssn", "123456789").value == "12*****89"
    assert loza.URL("https://loza.dev").key == "url"
    assert loza.EmailHash("test@test.com").key == "email.hash"
    assert loza.IPHash("10.0.0.1").key == "ip.hash"
