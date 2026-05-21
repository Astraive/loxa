use serde_json::Value;

pub fn encode_line(value: &Value) -> Result<String, serde_json::Error> {
    Ok(format!("{}\n", serde_json::to_string(value)?))
}

pub fn parse_lines(input: &str) -> Vec<Value> {
    input
        .lines()
        .filter_map(|line| serde_json::from_str(line).ok())
        .collect()
}
