/// Load SDK version from loxa-rs.yaml metadata file.
///
/// Falls back to a hardcoded default if the file cannot be found or parsed.

const FALLBACK_VERSION: &str = "0.2.5";

/// Read version from loxa-rs.yaml, searching standard locations.
/// Returns FALLBACK_VERSION if file not found or parsing fails.
fn load_version() -> String {
    let candidates = vec![
        std::path::PathBuf::from("loxa-rs.yaml"),
        // Relative to Cargo.toml (one level up from src/)
        std::path::PathBuf::from("../loxa-rs.yaml"),
        std::path::PathBuf::from("../../loxa-rs.yaml"),
    ];

    for path in &candidates {
        if let Ok(content) = std::fs::read_to_string(path) {
            for line in content.lines() {
                let trimmed = line.trim();
                if let Some(rest) = trimmed.strip_prefix("version:") {
                    let value = rest.trim().trim_matches(|c| c == '"' || c == '\'');
                    if !value.is_empty() {
                        return value.to_string();
                    }
                }
            }
        }
    }

    FALLBACK_VERSION.to_string()
}

/// SDK version loaded from loxa-rs.yaml at startup.
pub fn sdk_version() -> &'static str {
    // Use a once_cell-like pattern for thread-safe lazy initialization
    use std::sync::OnceLock;
    static VERSION: OnceLock<String> = OnceLock::new();
    VERSION.get_or_init(|| load_version())
}
