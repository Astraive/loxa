use std::collections::BTreeMap;
use std::io;
use std::time::Duration;

/// Low-level HTTP request for transport abstraction.
#[derive(Clone, Debug)]
pub struct HttpRequest {
    pub method: String,
    pub url: String,
    pub headers: BTreeMap<String, String>,
    pub body: Option<Vec<u8>>,
}

impl HttpRequest {
    pub fn new(method: impl Into<String>, url: impl Into<String>) -> Self {
        Self {
            method: method.into(),
            url: url.into(),
            headers: BTreeMap::new(),
            body: None,
        }
    }

    pub fn with_header(mut self, key: impl Into<String>, value: impl Into<String>) -> Self {
        self.headers.insert(key.into(), value.into());
        self
    }

    pub fn with_body(mut self, body: impl Into<Vec<u8>>) -> Self {
        self.body = Some(body.into());
        self
    }
}

/// Low-level HTTP response.
#[derive(Clone, Debug)]
pub struct HttpResponse {
    pub status_code: u16,
    pub headers: BTreeMap<String, String>,
    pub body: String,
}

/// Low-level HTTP transport using ureq.
pub struct Transport {
    agent: ureq::Agent,
}

impl Default for Transport {
    fn default() -> Self {
        Self::new()
    }
}

impl Transport {
    pub fn new() -> Self {
        Self {
            agent: ureq::AgentBuilder::new()
                .timeout(Duration::from_millis(2_000))
                .build(),
        }
    }

    pub fn with_timeout_ms(timeout_ms: u64) -> Self {
        Self {
            agent: ureq::AgentBuilder::new()
                .timeout(Duration::from_millis(timeout_ms))
                .build(),
        }
    }

    pub fn send(&self, request: &HttpRequest) -> io::Result<HttpResponse> {
        let mut req = match request.method.to_uppercase().as_str() {
            "GET" => self.agent.get(&request.url),
            "POST" => self.agent.post(&request.url),
            "PUT" => self.agent.put(&request.url),
            "DELETE" => self.agent.delete(&request.url),
            other => {
                return Err(io::Error::new(
                    io::ErrorKind::InvalidInput,
                    format!("unsupported method: {other}"),
                ))
            }
        };
        for (key, value) in &request.headers {
            req = req.set(key, value);
        }
        let response = if let Some(body) = &request.body {
            req.send_bytes(body)
        } else {
            req.call()
        };
        let response = response.map_err(io::Error::other)?;
        let status_code = response.status();
        let mut headers = BTreeMap::new();
        for name in &["content-type", "x-request-id", "x-deduped"] {
            if let Some(value) = response.header(name) {
                headers.insert(name.to_string(), value.to_string());
            }
        }
        let body = response.into_string().map_err(io::Error::other)?;
        Ok(HttpResponse {
            status_code,
            headers,
            body,
        })
    }
}
