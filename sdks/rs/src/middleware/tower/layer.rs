use super::service::MiddlewareConfig;

#[derive(Clone, Debug)]
pub struct LozaLayer {
    pub config: MiddlewareConfig,
}

impl LozaLayer {
    pub fn new(config: MiddlewareConfig) -> Self {
        Self { config }
    }
}
