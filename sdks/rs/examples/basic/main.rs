use loxa::{Config, Logger, Params, UserID};

fn main() {
    let logger = Logger::new(Config::dev("checkout"));
    let mut ctx = logger.start_event(Params::new("checkout.request"));
    logger.append(&mut ctx, UserID("u_123"));
    let _ = logger.finish(&mut ctx, "success");
    let _ = logger.emit(&ctx);
}
