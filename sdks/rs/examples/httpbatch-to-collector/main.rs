use loxa::{Config, New, Params};

fn main() {
    let logger = New(Config::production("checkout")
        .with_sink(loxa::HttpBatchSink("http://127.0.0.1:9090/events")));
    let mut ctx = logger.start_event(Params::new("checkout.collector"));
    let _ = logger.finish(&mut ctx, "success");
    let _ = logger.emit(&ctx);
}
