use crate::{config::SamplerConfig, EventContext};
use serde_json::Value;
use std::time::Duration;

pub fn should_sample(event: &EventContext, sampler: &SamplerConfig) -> bool {
    match sampler {
        SamplerConfig::All => true,
        SamplerConfig::None => false,
        SamplerConfig::Any(samplers) => {
            samplers.iter().any(|sampler| should_sample(event, sampler))
        }
        SamplerConfig::AllOf(samplers) => {
            samplers.iter().all(|sampler| should_sample(event, sampler))
        }
        SamplerConfig::Not(sampler) => !should_sample(event, sampler),
        SamplerConfig::Errors => event.outcome.as_deref() == Some("error") || event.error.is_some(),
        SamplerConfig::SlowRequests(ms) => event.duration_ms.unwrap_or_default() >= *ms,
        SamplerConfig::StatusCodes(codes) => event
            .status_code
            .map(|c| codes.contains(&c))
            .unwrap_or(false),
        SamplerConfig::Routes(routes) => event
            .route
            .as_ref()
            .map(|route| routes.contains(route))
            .unwrap_or(false),
        SamplerConfig::Users(ids) => event
            .user
            .get("id")
            .and_then(Value::as_str)
            .map(|id| ids.iter().any(|wanted| wanted == id))
            .unwrap_or(false),
        SamplerConfig::Tenants(ids) => event
            .tenant
            .get("id")
            .and_then(Value::as_str)
            .map(|id| ids.iter().any(|wanted| wanted == id))
            .unwrap_or(false),
        SamplerConfig::FeatureFlag(name, value) => event
            .attrs
            .get("feature")
            .and_then(Value::as_object)
            .and_then(|feature| feature.get(name))
            .map(|candidate| candidate == value)
            .unwrap_or(false),
        SamplerConfig::SampleRandom(rate) => {
            let bounded = (*rate).clamp(0.0, 1.0);
            fastrand::f64() < bounded
        }
        SamplerConfig::SampleRateLimited(rate, window) => {
            if *rate <= 0.0 || *window <= Duration::ZERO {
                return false;
            }
            // Simple rate limit: allow events at roughly rate/window frequency
            // (Note: This is a simplified approximation; actual token-bucket tracking would require state)
            let window_ms = window.as_millis() as f64;
            let rate_per_ms = *rate / window_ms;
            fastrand::f64() < rate_per_ms
        }
        SamplerConfig::Custom(f) => f(event),
        SamplerConfig::SampleByHeader(header, value) => {
            let normalized = header.to_lowercase().replace('_', "-");
            let keys = [
                format!("http.header.{normalized}"),
                format!("http.headers.{normalized}"),
                normalized.clone(),
            ];
            let want = value.trim();
            keys.iter().any(|key| {
                lookup_attr_string(event, key).map_or(false, |got| {
                    if want.is_empty() {
                        !got.trim().is_empty()
                    } else {
                        got.split(',')
                            .any(|part| part.trim().eq_ignore_ascii_case(want))
                    }
                })
            })
        }
    }
}

fn lookup_attr_string(event: &EventContext, key: &str) -> Option<String> {
    // Check in http group first
    if let Some(val) = event.http.get(key).and_then(Value::as_str) {
        return Some(val.to_string());
    }
    // Check in attrs
    if let Some(val) = event.attrs.get(key).and_then(Value::as_str) {
        return Some(val.to_string());
    }
    // Check nested keys with dot-separated paths
    let parts: Vec<&str> = key.splitn(2, '.').collect();
    if parts.len() == 2 {
        let group = event.attrs.get(parts[0]).and_then(Value::as_object);
        if let Some(obj) = group {
            if let Some(val) = obj.get(parts[1]).and_then(Value::as_str) {
                return Some(val.to_string());
            }
        }
    }
    None
}
