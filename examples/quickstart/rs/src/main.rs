fn main() {
    // Default API — configure once, use everywhere
    loza::configure(loza::Config::production("quickstart-demo").with_collector_endpoint("http://localhost:9308"))
        .expect("failed to configure");

    loza::info("server started");

    let ctx = loza::start_event(loza::Params::new("user.signup").with_kind("http"));
    loza::enrich(&ctx, "user.email", "demo@example.com");
    loza::enrich(&ctx, "user.plan", "pro");
    loza::finish(&ctx, "success");
    match loza::emit(&ctx) {
        Ok(result) => println!("Event emitted: {result}"),
        Err(e) => eprintln!("emit error: {e}"),
    }

    // Custom instance
    let logger = loza::create_loza(loza::Config::dev("checkout-api").with_collector_endpoint("http://localhost:9308"));
    logger.info("custom instance ready");

    // Alias — same config, different service name
    let audit = loza::alias("audit-service");
    audit.info("audit trail started");
}
