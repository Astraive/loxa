use loxa::{Config, Logger, Params, SchemaConfig};

fn main() {
    let logger = Logger::new(Config::test("checkout").with_schema(SchemaConfig::Flat));
    let mut ctx = logger.start_event(Params::new("checkout.flat"));
    let _ = logger.finish(&mut ctx, "success");
    println!("{}", logger.emit(&ctx).unwrap_or_default());
}
