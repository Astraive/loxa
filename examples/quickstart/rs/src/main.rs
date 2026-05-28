fn main() {
    // Default API — configure once, use everywhere
    loxa::configure(loxa::Config::production("quickstart-demo").with_collector_endpoint("http://localhost:9308"))
        .expect("failed to configure");

    loxa::info("server started");

    let ctx = loxa::start_event(loxa::Params::new("user.signup").with_kind("http"));
    loxa::enrich(&ctx, "user.email", "demo@example.com");
    loxa::enrich(&ctx, "user.plan", "pro");
    loxa::finish(&ctx, "success");
    match loxa::emit(&ctx) {
        Ok(result) => println!("Event emitted: {result}"),
        Err(e) => eprintln!("emit error: {e}"),
    }

    // Custom instance
    let logger = loxa::create_loxa(loxa::Config::dev("checkout-api").with_collector_endpoint("http://localhost:9308"));
    logger.info("custom instance ready");

    // Alias — same config, different service name
    let audit = loxa::alias("audit-service");
    audit.info("audit trail started");
}
