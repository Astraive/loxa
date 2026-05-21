pub mod canonical;
pub mod dotkey;
pub mod duplicate;
pub mod duplicate_policy;
pub mod level;
pub mod structure;
pub mod uuidv7;

pub const PIPELINE_ORDER: &[&str] = &[
    "StartEvent",
    "Append",
    "Checkpoint",
    "Finish",
    "sampling",
    "redaction",
    "schema",
    "sink",
];

pub fn pipeline_order() -> &'static [&'static str] {
    PIPELINE_ORDER
}
