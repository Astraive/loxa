use loxa::{Config, New, Params, SampleErrors};

pub fn sample_error_once() -> String {
    let logger = New(Config::test("bench").with_sampler(SampleErrors()));
    let mut ctx = logger.start_event(Params::new("bench.sampler"));
    logger.finish_error(&mut ctx, "boom");
    logger.emit(&ctx).unwrap_or_default()
}

