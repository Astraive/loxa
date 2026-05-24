use loxa::{Config, Params};
use std::time::Instant;

/// Run N emit iterations with auth configured. Returns (iterations, elapsed_ns, bytes_emitted).
pub fn run_auth_emit_iterations(iterations: usize) -> (usize, u128, usize) {
    let sink = loxa::MemorySink::new();
    let _logger = Config::test("bench")
        .with_sink(sink)
        .with_api_key("lx_sec_live_kBenchKey_bench_secret_value")
        .build()
        .expect("config");

    let mut bytes = 0;
    let start = Instant::now();
    for _ in 0..iterations {
        let ctx = loxa::start_event(Params::new("bench.auth.emit"));
        loxa::finish(&ctx, "success");
        if let Ok(encoded) = loxa::emit(&ctx) {
            bytes += encoded.len();
        }
    }
    let elapsed = start.elapsed().as_nanos();
    (iterations, elapsed, bytes)
}

/// Run N emit iterations with auth + enriched attributes. Returns (iterations, elapsed_ns, bytes_emitted).
pub fn run_auth_emit_enriched_iterations(iterations: usize) -> (usize, u128, usize) {
    let sink = loxa::MemorySink::new();
    let _logger = Config::test("bench")
        .with_sink(sink)
        .with_api_key("lx_sec_live_kBenchKey_bench_secret_value")
        .build()
        .expect("config");

    let mut bytes = 0;
    let start = Instant::now();
    for _ in 0..iterations {
        let ctx = loxa::start_event(Params::new("bench.auth.attrs"));
        loxa::set_string(&ctx, "http.method", "POST");
        loxa::set_string(&ctx, "http.path", "/api/payments");
        loxa::set_int(&ctx, "http.status", 200);
        loxa::set_float64(&ctx, "payment.amount", 99.99);
        loxa::set_bool(&ctx, "payment.success", true);
        loxa::finish(&ctx, "success");
        if let Ok(encoded) = loxa::emit(&ctx) {
            bytes += encoded.len();
        }
    }
    let elapsed = start.elapsed().as_nanos();
    (iterations, elapsed, bytes)
}

/// Run N emit iterations without auth (baseline). Returns (iterations, elapsed_ns, bytes_emitted).
pub fn run_no_auth_emit_iterations(iterations: usize) -> (usize, u128, usize) {
    let sink = loxa::MemorySink::new();
    let _logger = Config::test("bench")
        .with_sink(sink)
        .build()
        .expect("config");

    let mut bytes = 0;
    let start = Instant::now();
    for _ in 0..iterations {
        let ctx = loxa::start_event(Params::new("bench.baseline"));
        loxa::finish(&ctx, "success");
        if let Ok(encoded) = loxa::emit(&ctx) {
            bytes += encoded.len();
        }
    }
    let elapsed = start.elapsed().as_nanos();
    (iterations, elapsed, bytes)
}
