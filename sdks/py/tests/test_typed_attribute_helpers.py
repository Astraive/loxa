from __future__ import annotations

import hashlib

import loxa


def test_typed_attribute_helpers_snake_case():
    assert loxa.money("price", 1000, "USD").key == "price"
    assert loxa.percent("cpu", 85.5).key == "cpu"
    assert loxa.bytes_attr("file.size", 1024).key == "file.size"
    assert loxa.http_status("response.status", 200).key == "response.status"
    assert loxa.status_code("code", 404).value == 404
    assert loxa.error_code("err.code", "E123").value == "E123"
    assert loxa.bucket("user.tier", "premium").key == "user.tier"
    assert loxa.tags("env", "prod", "staging").key == "env"
    assert loxa.masked("card", "4111111111111111").value == "41************11"
    assert loxa.url("https://example.com").key == "url"
    assert loxa.email_hash("User@Example.COM").value == hashlib.sha256(b"user@example.com").hexdigest()
    assert loxa.ip_hash("192.168.1.1").value == hashlib.sha256(b"192.168.1.1").hexdigest()


def test_typed_attribute_helpers_pascal_case():
    assert loxa.Money("price", 500, "EUR").key == "price"
    assert loxa.Percent("mem", 72.3).key == "mem"
    assert loxa.Bytes("disk", 2048).key == "disk"
    assert loxa.HTTPStatus("status", 503).key == "status"
    assert loxa.StatusCode("code", 404).value == 404
    assert loxa.Bucket("env", "prod").key == "env"
    assert loxa.Tags("region", "us", "eu").key == "region"
    assert loxa.Masked("ssn", "123456789").value == "12*****89"
    assert loxa.URL("https://loxa.dev").key == "url"
    assert loxa.EmailHash("test@test.com").key == "email.hash"
    assert loxa.IPHash("10.0.0.1").key == "ip.hash"
