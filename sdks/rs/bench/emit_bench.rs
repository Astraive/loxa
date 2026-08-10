use loza::{Config, New, Params, String as LozaString};

pub fn run_emit_iterations(iterations: usize) -> usize {
    let logger = New(Config::test("bench"));
    let mut bytes = 0;
    for idx in 0..iterations {
        let mut ctx = logger.start_event(Params::new("bench.emit"));
        logger.append(&mut ctx, LozaString("iteration", &idx.to_string()));
        logger.finish(&mut ctx, "success");
        bytes += logger.emit(&ctx).unwrap_or_default().len();
    }
    bytes
}

