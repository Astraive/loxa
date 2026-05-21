pub fn escape_control_chars(value: &str) -> String {
    value.chars().filter(|ch| !ch.is_control()).collect()
}

pub fn truncate_utf8(value: &str, max_bytes: usize) -> String {
    if value.len() <= max_bytes {
        return value.to_string();
    }
    let mut end = 0;
    for (idx, _) in value.char_indices() {
        if idx <= max_bytes {
            end = idx;
        } else {
            break;
        }
    }
    value[..end].to_string()
}
