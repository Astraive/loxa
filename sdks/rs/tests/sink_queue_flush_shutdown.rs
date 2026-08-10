#[test]
fn sink_queue_flush_shutdown_helpers_work() {
    let memory = loza::memory_sink();
    let noop = loza::noop_sink();
    let _stdout = loza::stdout_sink();
    let _otlp = loza::otlp_sink("http://127.0.0.1:4318");
    let sinks = loza::multi_sink(&[memory.clone(), noop]);
    assert_eq!(sinks.len(), 2);
    loza::drain(&memory);
    loza::pause(&memory);
    loza::resume(&memory);
    assert_eq!(loza::queue_size(&memory), 0);
    assert!(loza::health(&memory));
    loza::flush();
    loza::shutdown();
}
