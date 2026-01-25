#![allow(dead_code)]
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

use sdk::syscalls::{HostPayload, HostResponse, SyscallClient};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum TransportProtocol {
    Http,
    WebSocket,
    WebRtc,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum RequestPattern {
    Unary,
    ServerStreaming,
    ClientStreaming,
    Bidirectional,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiRequest {
    pub protocol: TransportProtocol,
    pub pattern: RequestPattern,
    pub provider: String,
    pub endpoint: String,
    pub method: String,
    pub headers: HashMap<String, String>,
    pub body: Vec<u8>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiResponse {
    pub status: u16,
    pub headers: HashMap<String, String>,
    pub body: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct ApiMeta {
    protocol: TransportProtocol,
    pattern: RequestPattern,
    provider: String,
    endpoint: String,
    method: String,
    headers: HashMap<String, String>,
}

#[derive(Debug, Serialize, Deserialize)]
struct ApiResponseMeta {
    status: u16,
    headers: HashMap<String, String>,
}

pub struct ApiProxy;

impl ApiProxy {
    pub fn new() -> Self {
        Self
    }

    pub async fn call_api(&self, request: ApiRequest) -> Result<ApiResponse, String> {
        let sab = crate::get_cached_sab().ok_or_else(|| "Shared SAB not initialized".to_string())?;

        let meta = ApiMeta {
            protocol: request.protocol.clone(),
            pattern: request.pattern.clone(),
            provider: request.provider.clone(),
            endpoint: request.endpoint.clone(),
            method: request.method.clone(),
            headers: request.headers.clone(),
        };

        let custom = serde_json::to_vec(&meta).map_err(|e| e.to_string())?;

        let response = SyscallClient::host_call(
            &sab,
            "api.request",
            HostPayload::Inline(&request.body),
            Some(&custom),
        )
        .await
        .map_err(|e| format!("Host API request failed: {}", e))?;

        let (body, custom_bytes) = match response {
            HostResponse::Inline { data, custom } => (data, custom),
            HostResponse::SabRef { offset, size, custom } => {
                let mut data = vec![0u8; size as usize];
                sab.read_raw(offset as usize, &mut data)
                    .map_err(|e| format!("Failed reading SAB response: {}", e))?;
                (data, custom)
            }
        };

        let meta: ApiResponseMeta = if custom_bytes.is_empty() {
            ApiResponseMeta {
                status: 200,
                headers: HashMap::new(),
            }
        } else {
            serde_json::from_slice(&custom_bytes).map_err(|e| e.to_string())?
        };

        Ok(ApiResponse {
            status: meta.status,
            headers: meta.headers,
            body,
        })
    }

    pub async fn openai_chat(
        &self,
        api_key: &str,
        model: &str,
        messages: Vec<(String, String)>,
    ) -> Result<String, String> {
        let mut headers = HashMap::new();
        headers.insert("Authorization".to_string(), format!("Bearer {}", api_key));
        headers.insert("Content-Type".to_string(), "application/json".to_string());

        let messages_json: Vec<serde_json::Value> = messages
            .iter()
            .map(|(role, content)| {
                serde_json::json!({
                    "role": role,
                    "content": content
                })
            })
            .collect();

        let body = serde_json::json!({
            "model": model,
            "messages": messages_json
        });

        let request = ApiRequest {
            protocol: TransportProtocol::Http,
            pattern: RequestPattern::Unary,
            provider: "openai".to_string(),
            endpoint: "chat/completions".to_string(),
            method: "POST".to_string(),
            headers,
            body: serde_json::to_vec(&body).map_err(|e| e.to_string())?,
        };

        let response = self.call_api(request).await?;
        String::from_utf8(response.body).map_err(|e| e.to_string())
    }
}
