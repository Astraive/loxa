// Integration wrapper for Rust SDK conformance
// Re-exports all e2e tests to match conformance runner expectations
mod e2e;

// The conformance runner will find this test module via `cargo test --test integration`
