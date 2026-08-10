use loza::{Config, New, Params};

fn main() {
    let logger = New(Config::production("checkout")
        .with_sink(loza::HttpBatchSink("http://127.0.0.1:9308/events")));
    let mut ctx = logger.start_event(Params::new("checkout.collector"));
    let _ = logger.finish(&mut ctx, "success");
    let _ = logger.emit(&ctx);
}
