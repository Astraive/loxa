
pub fn middleware_name() -> &'static str {
    "loxa-actix"
}

#[cfg(feature = "actix")]
pub mod actix_impl {
    use super::*;
    use actix_web::dev::{forward_ready, Service, ServiceRequest, ServiceResponse, Transform};
    use actix_web::Error;
    use futures_util::future::{ok, LocalBoxFuture, Ready};
    use std::sync::Arc;
    use std::time::Instant;

    pub struct LoxaMiddleware {
        logger: Arc<Logger>,
        service_name: String,
    }

    impl LoxaMiddleware {
        pub fn new(logger: Logger, service_name: impl Into<String>) -> Self {
            Self {
                logger: Arc::new(logger),
                service_name: service_name.into(),
            }
        }
    }

    impl<S, B> Transform<S, ServiceRequest> for LoxaMiddleware
    where
        S: Service<ServiceRequest, Response = ServiceResponse<B>, Error = Error>,
        S::Future: 'static,
        B: 'static,
    {
        type Response = ServiceResponse<B>;
        type Error = Error;
        type Transform = LoxaService<S>;
        type InitError = ();
        type Future = Ready<Result<Self::Transform, Self::InitError>>;

        fn new_transform(&self, service: S) -> Self::Future {
            ok(LoxaService {
                service,
                logger: self.logger.clone(),
                service_name: self.service_name.clone(),
            })
        }
    }

    pub struct LoxaService<S> {
        service: S,
        logger: Arc<Logger>,
        service_name: String,
    }

    impl<S, B> Service<ServiceRequest> for LoxaService<S>
    where
        S: Service<ServiceRequest, Response = ServiceResponse<B>, Error = Error>,
        S::Future: 'static,
        B: 'static,
    {
        type Response = ServiceResponse<B>;
        type Error = Error;
        type Future = LocalBoxFuture<'static, Result<Self::Response, Self::Error>>;

        forward_ready!(service);

        fn call(&self, req: ServiceRequest) -> Self::Future {
            let logger = self.logger.clone();
            let service_name = self.service_name.clone();
            let method = req.method().to_string();
            let path = req.path().to_string();
            let user_agent = req
                .headers()
                .get("user-agent")
                .and_then(|v| v.to_str().ok())
                .unwrap_or("")
                .to_string();
            let remote_ip = req.peer_addr().map(|a| a.to_string()).unwrap_or_default();

            let started = Instant::now();
            let fut = self.service.call(req);

            Box::pin(async move {
                let mut params = Params::new("http.request").with_kind("http");
                params.method = Some(method);
                params.path = Some(path.clone());
                params.route = Some(path);
                params.service = Some(service_name);
                let mut ctx = logger.start_event(params);

                crate::Append(&mut ctx, crate::String("http.user_agent", user_agent));
                crate::Append(&mut ctx, crate::String("http.remote_ip", remote_ip));

                match fut.await {
                    Ok(res) => {
                        let status = res.status().as_u16();
                        let elapsed = started.elapsed().as_millis() as u64;
                        let outcome = if status >= 500 { "error" } else { "success" };
                        crate::Append(&mut ctx, crate::Int("status_code", status as i64));
                        crate::Append(&mut ctx, crate::Int("duration_ms", elapsed as i64));
                        let _ = logger.finish(&mut ctx, outcome);
                        let _ = logger.emit(&ctx);
                        Ok(res)
                    }
                    Err(err) => {
                        let _ = logger.finish_error(&mut ctx, &err.to_string());
                        let _ = logger.emit(&ctx);
                        Err(err)
                    }
                }
            })
        }
    }
}
