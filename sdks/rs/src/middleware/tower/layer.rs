use super::service::MiddlewareConfig;

#[derive(Clone, Debug)]
pub struct LoxaLayer {
    pub config: MiddlewareConfig,
}

impl LoxaLayer {
    pub fn new(config: MiddlewareConfig) -> Self {
        Self { config }
    }
}
