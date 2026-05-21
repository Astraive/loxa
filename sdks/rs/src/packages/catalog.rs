pub const MIDDLEWARE_PACKAGES: &[&str] = &["axum", "actix", "tower", "hyper"];
pub const INTEGRATION_PACKAGES: &[&str] = &["otel", "tracing", "log"];
pub const SINK_PACKAGES: &[&str] = &["httpbatch"];

pub fn all_packages() -> Vec<&'static str> {
    MIDDLEWARE_PACKAGES
        .iter()
        .chain(INTEGRATION_PACKAGES)
        .chain(SINK_PACKAGES)
        .copied()
        .collect()
}
