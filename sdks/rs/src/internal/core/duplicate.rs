use serde_json::Value;

use super::duplicate_policy::DuplicatePolicy;

pub fn resolve_duplicate(
    existing: Value,
    incoming: Value,
    policy: DuplicatePolicy,
) -> Result<Value, String> {
    match policy {
        DuplicatePolicy::CanonicalWins | DuplicatePolicy::FirstWins => Ok(existing),
        DuplicatePolicy::UserWins | DuplicatePolicy::LastWins => Ok(incoming),
        DuplicatePolicy::KeepBoth => {
            let result = match existing {
                Value::Array(mut values) => {
                    values.push(incoming);
                    Value::Array(values)
                }
                other => Value::Array(vec![other, incoming]),
            };
            Ok(result)
        }
        DuplicatePolicy::ErrorOnDuplicate => {
            Err("duplicate field rejected by ErrorOnDuplicate policy".to_string())
        }
    }
}
