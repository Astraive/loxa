use crate::{Logger, Params};

#[derive(Clone, Debug)]
pub struct MiddlewareConfig {
    pub service: String,
    pub route: Option<String>,
    pub recover_panics: bool,
}

impl Default for MiddlewareConfig {
    fn default() -> Self {
        Self {
            service: String::new(),
            route: None,
            recover_panics: true,
        }
    }
}

#[derive(Clone, Debug)]
pub struct MiddlewareResult {
    pub event_id: String,
    pub encoded: String,
}

pub fn capture_request(
    logger: &Logger,
    method: &str,
    path: &str,
    status_code: u16,
) -> Result<MiddlewareResult, serde_json::Error> {
    let mut params = Params::new(format!("{method} {path}")).with_kind("http");
    params.method = Some(method.to_string());
    params.path = Some(path.to_string());
    params.status_code = Some(status_code);
    let mut ctx = logger.start_event(params);
    let _ = logger.finish(
        &mut ctx,
        if status_code >= 500 {
            "error"
        } else {
            "success"
        },
    );
    let event_id = ctx.event_id.clone();
    let encoded = logger
        .emit(&ctx)
        .map_err(|err| serde_json::Error::io(std::io::Error::other(err.to_string())))?;
    Ok(MiddlewareResult { event_id, encoded })
}
