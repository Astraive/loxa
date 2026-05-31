pub fn middleware_name() -> &'static str {
    "loxa-axum"
}

#[cfg(feature = "axum")]
pub mod axum_impl {
    use crate::{Logger, Params};
    use axum::extract::Request;
    use axum::middleware::Next;
    use axum::response::Response;
    use std::sync::Arc;
    use std::time::Instant;

    #[allow(dead_code)]
    pub struct LoxaLayer {
        logger: Arc<Logger>,
        service_name: String,
    }

    impl LoxaLayer {
        pub fn new(logger: Logger, service_name: impl Into<String>) -> Self {
            Self {
                logger: Arc::new(logger),
                service_name: service_name.into(),
            }
        }
    }

    pub async fn loxa_middleware(
        logger: Arc<Logger>,
        service_name: String,
        req: Request,
        next: Next,
    ) -> Response {
        let method = req.method().to_string();
        let path = req.uri().path().to_string();
        let user_agent = req
            .headers()
            .get("user-agent")
            .and_then(|v| v.to_str().ok())
            .unwrap_or("")
            .to_string();
        let remote_ip = ""; // Axum doesn't expose peer addr easily in middleware

        let started = Instant::now();

        let mut params = Params::new("http.request").with_kind("http");
        params.method = Some(method);
        params.path = Some(path.clone());
        params.route = Some(path);
        params.service = Some(service_name);
        let mut ctx = logger.start_event(params);

        crate::Append(&mut ctx, crate::String("http.user_agent", user_agent));
        crate::Append(&mut ctx, crate::String("http.remote_ip", remote_ip));

        let res = next.run(req).await;

        let status = res.status().as_u16();
        let elapsed = started.elapsed().as_millis() as u64;
        let outcome = if status >= 500 { "error" } else { "success" };
        crate::Append(&mut ctx, crate::Int("status_code", status as i64));
        crate::Append(&mut ctx, crate::Int("duration_ms", elapsed as i64));
        let _ = logger.finish(&mut ctx, outcome);
        let _ = logger.emit(&ctx);

        res
    }
}
