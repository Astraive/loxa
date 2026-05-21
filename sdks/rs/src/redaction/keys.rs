use sha2::{Digest, Sha256};

// Safety-net keys for obviously sensitive fields. The collector owns the real
// PII policy (email, phone, SSN, IP, regex, tenant-specific rules, etc.).
pub const DEFAULT_KEYS: &[&str] = &[
    "password",
    "passwd",
    "pwd",
    "secret",
    "token",
    "access_token",
    "refresh_token",
    "api_key",
    "apikey",
    "auth",
    "authorization",
    "credential",
    "private_key",
    "client_secret",
];

pub fn matches_key(key: &str, keys: &[&str]) -> bool {
    let lowered = key.to_ascii_lowercase();
    keys.iter().any(|wanted| {
        lowered == *wanted
            || lowered.rsplit('.').next() == Some(*wanted)
            || lowered.rsplit('_').next() == Some(*wanted)
    })
}

pub fn sha256_hex(value: &str) -> String {
    let mut hasher = Sha256::new();
    hasher.update(value.as_bytes());
    let digest = hasher.finalize();
    let mut out = String::with_capacity(digest.len() * 2);
    for byte in digest {
        out.push_str(&format!("{byte:02x}"));
    }
    out
}

pub fn hash_input(value: &serde_json::Value) -> String {
    match value {
        serde_json::Value::String(text) => text.clone(),
        _ => value.to_string(),
    }
}
