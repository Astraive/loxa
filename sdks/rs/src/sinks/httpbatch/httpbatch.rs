use crate::core::client::CollectorHttpClient;
use crate::generated::spec_contract::CollectorResponse;
use crate::SinkConfig;
use serde_json::Value;

#[derive(Clone, Debug)]
pub struct HttpBatchSinkConfig {
    pub endpoint: String,
    pub api_key: Option<String>,
    pub timeout_ms: u64,
    pub max_batch_bytes: usize,
}

impl HttpBatchSinkConfig {
    pub fn new(endpoint: impl Into<String>) -> Self {
        Self {
            endpoint: endpoint.into(),
            api_key: None,
            timeout_ms: 2_000,
            max_batch_bytes: 256 * 1024,
        }
    }

    pub fn with_api_key(mut self, api_key: impl Into<String>) -> Self {
        self.api_key = Some(api_key.into());
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
            timeout_ms: self.timeout_ms,
            max_batch_bytes: self.max_batch_bytes,
            max_retries: 3,
            enable_compression: true,
            ndjson: false,
        }
    }

    pub fn client(&self) -> CollectorHttpClient {
        let mut client = CollectorHttpClient::new(self.endpoint.clone());
        client.timeout_ms = self.timeout_ms;
        if let Some(api_key) = &self.api_key {
            client = client.with_api_key(api_key.clone());
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
