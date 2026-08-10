use loza::{Config, New, Params};

pub fn encode_once() -> String {
    let logger = New(Config::test("bench"));
    let mut ctx = logger.start_event(Params::new("bench.encoder"));
    logger.finish(&mut ctx, "success");
    logger.emit(&ctx).unwrap_or_default()
}

