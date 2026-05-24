use loxa::{Config, New, Params, SchemaConfig};

fn main() {
    let logger = New(Config::test("checkout").with_schema(SchemaConfig::Flat));
    let mut ctx = logger.start_event(Params::new("checkout.flat"));
    let _ = logger.finish(&mut ctx, "success");
    println!("{}", logger.emit(&ctx).unwrap_or_default());
}
