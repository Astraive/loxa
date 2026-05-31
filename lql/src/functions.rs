use crate::ast::{Duration, DurationUnit};
use std::time::{SystemTime, UNIX_EPOCH};

/// Compute the timestamp for `ago(duration)` — current time minus the duration.
pub fn ago_millis(d: &Duration) -> u64 {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64;
    now.saturating_sub(d.to_millis())
}

/// Format a duration as a DuckDB INTERVAL literal.
pub fn duration_to_duckdb_interval(d: &Duration) -> String {
    match d.unit {
        DurationUnit::Milliseconds => format!("INTERVAL '{}' MILLISECOND", d.value),
        DurationUnit::Seconds => format!("INTERVAL '{}' SECOND", d.value),
        DurationUnit::Minutes => format!("INTERVAL '{}' MINUTE", d.value),
        DurationUnit::Hours => format!("INTERVAL '{}' HOUR", d.value),
        DurationUnit::Days => format!("INTERVAL '{}' DAY", d.value),
        DurationUnit::Weeks => format!("INTERVAL '{}' DAY", d.value.saturating_mul(7)),
    }
}

/// Format a duration as a ClickHouse INTERVAL literal.
pub fn duration_to_clickhouse_interval(d: &Duration) -> String {
    match d.unit {
        DurationUnit::Milliseconds => format!("toIntervalMillisecond({})", d.value),
        DurationUnit::Seconds => format!("toIntervalSecond({})", d.value),
        DurationUnit::Minutes => format!("toIntervalMinute({})", d.value),
        DurationUnit::Hours => format!("toIntervalHour({})", d.value),
        DurationUnit::Days => format!("toIntervalDay({})", d.value),
        DurationUnit::Weeks => format!("toIntervalDay({})", d.value.saturating_mul(7)),
    }
}

/// Known built-in functions and their signatures.
pub fn is_builtin_function(name: &str) -> bool {
    matches!(
        name.to_lowercase().as_str(),
        "ago"
            | "bin"
            | "now"
            | "coalesce"
            | "if"
            | "isempty"
            | "isnotempty"
            | "strlen"
            | "tolower"
            | "toupper"
            | "trim"
            | "substring"
            | "replace"
            | "split"
            | "array_length"
            | "typeof"
            | "to_string"
            | "to_int"
            | "to_float"
            | "abs"
            | "round"
            | "floor"
            | "ceil"
            | "log"
            | "sqrt"
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ago_1h() {
        let d = Duration {
            value: 1,
            unit: DurationUnit::Hours,
        };
        let ts = ago_millis(&d);
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_millis() as u64;
        assert!(ts < now);
        assert!(now - ts >= 3_600_000 - 1000); // within 1s tolerance
    }

    #[test]
    fn duration_to_intervals() {
        let d = Duration {
            value: 5,
            unit: DurationUnit::Minutes,
        };
        assert_eq!(duration_to_duckdb_interval(&d), "INTERVAL '5' MINUTE");
        assert_eq!(duration_to_clickhouse_interval(&d), "toIntervalMinute(5)");
    }
}
