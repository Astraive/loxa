pub fn split_dot_key(key: &str) -> Vec<&str> {
    key.split('.').filter(|part| !part.is_empty()).collect()
}

pub fn snake_key(key: &str) -> String {
    key.replace('.', "_")
}

pub fn camel_key(key: &str) -> String {
    let mut out = String::new();
    for (idx, part) in split_dot_key(key).into_iter().enumerate() {
        if idx == 0 {
            out.push_str(part);
        } else {
            let mut chars = part.chars();
            if let Some(first) = chars.next() {
                out.push(first.to_ascii_uppercase());
                out.push_str(chars.as_str());
            }
        }
    }
    out
}
