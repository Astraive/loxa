use loza::{Config, Params};
use std::time::Instant;

/// Run N emit iterations with auth configured. Returns (iterations, elapsed_ns, bytes_emitted).
pub fn run_auth_emit_iterations(iterations: usize) -> (usize, u128, usize) {
    let sink = loza::MemorySink::new();
    let _logger = Config::test("bench")
        .with_sink(sink)
        .with_api_key("lz_sec_live_kBenchKey_bench_secret_value")
        .build()
        .expect("config");

    let mut bytes = 0;
    let start = Instant::now();
    for _ in 0..iterations {
        let ctx = loza::start_event(Params::new("bench.auth.emit"));
        loza::finish(&ctx, "success");
        if let Ok(encoded) = loza::emit(&ctx) {
            bytes += encoded.len();
        }
    }
    let elapsed = start.elapsed().as_nanos();
    (iterations, elapsed, bytes)
}

/// Run N emit iterations with auth + enriched attributes. Returns (iterations, elapsed_ns, bytes_emitted).
pub fn run_auth_emit_enriched_iterations(iterations: usize) -> (usize, u128, usize) {
    let sink = loza::MemorySink::new();
    let _logger = Config::test("bench")
        .with_sink(sink)
        .with_api_key("lz_sec_live_kBenchKey_bench_secret_value")
        .build()
        .expect("config");

    let mut bytes = 0;
    let start = Instant::now();
    for _ in 0..iterations {
        let ctx = loza::start_event(Params::new("bench.auth.attrs"));
        loza::set_string(&ctx, "http.method", "POST");
        loza::set_string(&ctx, "http.path", "/api/payments");
        loza::set_int(&ctx, "http.status", 200);
        loza::set_float64(&ctx, "payment.amount", 99.99);
        loza::set_bool(&ctx, "payment.success", true);
        loza::finish(&ctx, "success");
        if let Ok(encoded) = loza::emit(&ctx) {
            bytes += encoded.len();
        }
    }
    let elapsed = start.elapsed().as_nanos();
    (iterations, elapsed, bytes)
}

/// Run N emit iterations without auth (baseline). Returns (iterations, elapsed_ns, bytes_emitted).
pub fn run_no_auth_emit_iterations(iterations: usize) -> (usize, u128, usize) {
    let sink = loza::MemorySink::new();
    let _logger = Config::test("bench")
        .with_sink(sink)
        .build()
        .expect("config");

    let mut bytes = 0;
    let start = Instant::now();
    for _ in 0..iterations {
        let ctx = loza::start_event(Params::new("bench.baseline"));
        loza::finish(&ctx, "success");
        if let Ok(encoded) = loza::emit(&ctx) {
            bytes += encoded.len();
        }
    }
    let elapsed = start.elapsed().as_nanos();
    (iterations, elapsed, bytes)
}
