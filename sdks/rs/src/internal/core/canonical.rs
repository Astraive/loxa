pub use crate::generated::spec_contract::CANONICAL_FIELDS;

pub fn is_canonical(key: &str) -> bool {
    crate::generated::spec_contract::is_canonical(key)
}
