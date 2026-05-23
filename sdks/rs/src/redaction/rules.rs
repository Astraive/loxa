use serde_json::Value;

use crate::config::RedactorConfig;

use super::keys::{hash_input, matches_key, sha256_hex, DEFAULT_KEYS};

pub fn redact(mut value: Value, redactor: &RedactorConfig) -> Value {
    match redactor {
        RedactorConfig::Default => redact_keys(&mut value, DEFAULT_KEYS, RedactionMode::Replace),
        RedactorConfig::Keys(keys) => {
            let refs: Vec<_> = keys.iter().map(String::as_str).collect();
            redact_keys(&mut value, &refs, RedactionMode::Replace)
        }
        RedactorConfig::HashKeys(keys) => {
            let refs: Vec<_> = keys.iter().map(String::as_str).collect();
            redact_keys(&mut value, &refs, RedactionMode::Hash)
        }
        RedactorConfig::DropKeys(keys) => {
            let refs: Vec<_> = keys.iter().map(String::as_str).collect();
            redact_keys(&mut value, &refs, RedactionMode::Drop)
        }
        RedactorConfig::MaskKeys(keys) => {
            let refs: Vec<_> = keys.iter().map(String::as_str).collect();
            redact_keys(&mut value, &refs, RedactionMode::Mask)
        }
        RedactorConfig::Patterns(patterns) => {
            redact_patterns(&mut value, patterns);
            value
        }
        RedactorConfig::Compose(redactors) => redactors.iter().fold(value, redact),
        RedactorConfig::AllowKeys(_keys) => value,
        RedactorConfig::None => value,
    }
}

pub enum RedactionMode {
    Replace,
    Hash,
    Drop,
    Mask,
}

fn redact_keys(value: &mut Value, keys: &[&str], mode: RedactionMode) -> Value {
    walk(value, keys, &mode);
    value.clone()
}

fn redact_patterns(value: &mut Value, patterns: &[String]) {
    let compiled: Vec<regex::Regex> = patterns
        .iter()
        .filter_map(|p| regex::Regex::new(p).ok())
        .collect();
    walk_patterns(value, &compiled);
}

fn walk_patterns(value: &mut Value, patterns: &[regex::Regex]) {
    match value {
        Value::Object(map) => {
            for (_key, val) in map.iter_mut() {
                if let Value::String(s) = val {
                    for pat in patterns {
                        if pat.is_match(s) {
                            *val = Value::String("[REDACTED]".to_string());
                            break;
                        }
                    }
                } else {
                    walk_patterns(val, patterns);
                }
            }
        }
        Value::Array(values) => {
            for child in values {
                walk_patterns(child, patterns);
            }
        }
        _ => {}
    }
}

fn walk(value: &mut Value, keys: &[&str], mode: &RedactionMode) {
    match value {
        Value::Object(map) => {
            let names: Vec<_> = map.keys().cloned().collect();
            for key in names {
                if matches_key(&key, keys) {
                    match mode {
                        RedactionMode::Replace => {
                            map.insert(key, Value::String("[REDACTED]".to_string()));
                        }
                        RedactionMode::Hash => {
                            let hashed = sha256_hex(&hash_input(&map[&key]));
                            map.insert(key, Value::String(hashed));
                        }
                        RedactionMode::Drop => {
                            map.remove(&key);
                        }
                        RedactionMode::Mask => {
                            let text = hash_input(&map[&key]);
                            let masked = if text.len() <= 4 {
                                "****".to_string()
                            } else {
                                let chars: Vec<char> = text.chars().collect();
                                let prefix: String = chars[..2].iter().collect();
                                let suffix: String = chars[chars.len() - 2..].iter().collect();
                                let stars = "*".repeat(chars.len() - 4);
                                format!("{prefix}{stars}{suffix}")
                            };
                            map.insert(key, Value::String(masked));
                        }
                    }
                } else if let Some(child) = map.get_mut(&key) {
                    walk(child, keys, mode);
                }
            }
        }
        Value::Array(values) => {
            for child in values {
                walk(child, keys, mode);
            }
        }
        _ => {}
    }
}
