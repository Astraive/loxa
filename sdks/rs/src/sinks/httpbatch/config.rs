use crate::core::client::CollectorHttpClient;
use crate::generated::spec_contract::CollectorResponse;
use crate::SinkConfig;
use serde_json::Value;

#[derive(Clone)]
pub struct HttpBatchSinkConfig {
    pub endpoint: String,
    pub api_key: Option<String>,
    pub basic_username: Option<String>,
    pub basic_password: Option<String>,
    pub insecure: bool,
    pub timeout_ms: u64,
    pub max_batch_bytes: usize,
}

impl std::fmt::Debug for HttpBatchSinkConfig {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("HttpBatchSinkConfig")
            .field("endpoint", &self.endpoint)
            .field("api_key", &self.api_key.as_ref().map(|_| "<redacted>"))
            .field(
                "basic_credentials",
                &self.basic_username.as_ref().map(|_| "<redacted>"),
            )
            .field("timeout_ms", &self.timeout_ms)
            .field("max_batch_bytes", &self.max_batch_bytes)
            .finish()
    }
}

impl HttpBatchSinkConfig {
    pub fn new(endpoint: impl Into<String>) -> Self {
        Self {
            endpoint: endpoint.into(),
            api_key: None,
            basic_username: None,
            basic_password: None,
            insecure: false,
            timeout_ms: 2_000,
            max_batch_bytes: 256 * 1024,
        }
    }

    pub fn with_api_key(mut self, api_key: impl Into<String>) -> Self {
        self.api_key = Some(api_key.into());
        self
    }
    pub fn with_basic_auth(
        mut self,
        username: impl Into<String>,
        password: impl Into<String>,
    ) -> Self {
        self.basic_username = Some(username.into());
        self.basic_password = Some(password.into());
        self
    }

    pub fn with_insecure(mut self, insecure: bool) -> Self {
        self.insecure = insecure;
        self
    }

    pub fn with_timeout_ms(mut self, timeout_ms: u64) -> Self {
        self.timeout_ms = timeout_ms;
        self
    }

    pub fn as_sink_config(&self) -> SinkConfig {
        SinkConfig::HttpBatch {
            endpoint: self.endpoint.clone(),
            api_key: self.api_key.clone(),
            basic_username: self.basic_username.clone(),
            basic_password: self.basic_password.clone(),
            insecure: self.insecure,
            timeout_ms: self.timeout_ms,
            max_batch_bytes: self.max_batch_bytes,
            max_retries: 3,
            enable_compression: true,
            ndjson: false,
        }
    }

    pub fn client(&self) -> CollectorHttpClient {
        let mut client =
            CollectorHttpClient::new(self.endpoint.clone()).with_insecure(self.insecure);
        client.timeout_ms = self.timeout_ms;
        if let Some(api_key) = &self.api_key {
            client = client.with_api_key(api_key.clone());
        } else if let (Some(username), Some(password)) =
            (&self.basic_username, &self.basic_password)
        {
            client = client.with_basic_auth(username.clone(), password.clone());
        }
        client
    }

    pub fn envelope(&self, encoded_event: &str) -> Value {
        self.batch_envelope(&[encoded_event.to_string()])
    }

    pub fn batch_envelope(&self, encoded_events: &[String]) -> Value {
        self.client().envelope(encoded_events)
    }

    pub fn parse_ack(&self, body: &str) -> Result<CollectorResponse, serde_json::Error> {
        self.client().parse_response(body)
    }
}
