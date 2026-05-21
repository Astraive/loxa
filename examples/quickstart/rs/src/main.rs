use loxa_rs::{Config, Logger, Params};

fn main() {
    let cfg = Config::production("quickstart-demo").with_collector_endpoint("http://localhost:9090");
    let logger = Logger::new(cfg).expect("failed to create logger");

    let ctx = logger.start_event(Params {
        event: "user.signup".into(),
        kind: "http".into(),
        service: "quickstart-demo".into(),
        ..Default::default()
    });

    logger.enrich(&ctx, "user.email", "demo@example.com");
    logger.enrich(&ctx, "user.plan", "pro");

    logger.finish(&ctx, "success");

    match logger.emit(&ctx) {
        Ok(result) => println!("Event emitted: {result}"),
        Err(e) => eprintln!("emit error: {e}"),
    }
}
