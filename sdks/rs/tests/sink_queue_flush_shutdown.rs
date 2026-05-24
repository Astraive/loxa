#[test]
fn sink_queue_flush_shutdown_helpers_work() {
    let memory = loxa::memory_sink();
    let noop = loxa::noop_sink();
    let _stdout = loxa::stdout_sink();
    let _otlp = loxa::otlp_sink("http://127.0.0.1:4318");
    let sinks = loxa::multi_sink(&[memory.clone(), noop]);
    assert_eq!(sinks.len(), 2);
    loxa::drain(&memory);
    loxa::pause(&memory);
    loxa::resume(&memory);
    assert_eq!(loxa::queue_size(&memory), 0);
    assert!(loxa::health(&memory));
    loxa::flush();
    loxa::shutdown();
}
