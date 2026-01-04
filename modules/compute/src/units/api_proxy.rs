use crate::engine::{ComputeError, ResourceLimits, UnitProxy};
use async_trait::async_trait;
use sdk::shader_registry::ShaderManifest;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use wasm_bindgen_futures::JsFuture;
use web_sys::{
    wasm_bindgen::JsCast, Request, RequestInit, RequestMode, Response, RtcConfiguration,
    RtcIceServer, RtcPeerConnection,
};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum TransportProtocol {
    Http,
    WebSocket,
    WebRtc,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum RequestPattern {
    Unary,           // One-off request/response
    ServerStreaming, // One request, multiple responses (Pub/Sub)
    ClientStreaming, // Multiple requests, one response
    Bidirectional,   // Full duplex
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

/// **Web Proxy Philosophy**: Don't implement heavy ML inference in-browser.
/// Instead, use a multi-layered transport strategy:
/// 1. **Centralized HTTP**: OpenAI, Anthropic, HuggingFace.
/// 2. **Real-time Sync (WebSockets)**: Pub/Sub for cross-node state synchronization.
/// 3. **Direct P2P (WebRTC)**: Low-latency data channels for federated compute.
///
/// **Request Patterns**:
/// - `Unary`: standard Request/Response.
/// - `ServerStreaming`: Listen for continuous updates (e.g. log streams, orbit telemetry).
/// - `ClientStreaming`: Send multiple data chunks (e.g. large file uploads, sensor bursts).
/// - `Bidirectional`: Full duplex coordination for P2P protocols.
///
/// **Sources**:
/// - `Cloud Providers`: Managed ML services.
/// - `Edge Nodes`: Private local hubs or other InOS instances.
/// - `Federated Grid`: P2P network of browsers acting as a collective.
pub struct ApiProxy {
    // Future: Add caching, rate limiting, retry logic
}

impl ApiProxy {
    pub fn new() -> Self {
        Self {}
    }

    /// Call external API
    ///
    /// # Examples
    ///
    /// OpenAI Chat Completion:
    /// ```json
    /// {
    ///   "provider": "openai",
    ///   "endpoint": "chat/completions",
    ///   "method": "POST",
    ///   "headers": {
    ///     "Authorization": "Bearer sk-...",
    ///     "Content-Type": "application/json"
    ///   },
    ///   "body": {
    ///     "model": "gpt-4",
    ///     "messages": [{"role": "user", "content": "Hello"}]
    ///   }
    /// }
    /// ```
    pub async fn call_api(&self, request: ApiRequest) -> Result<ApiResponse, String> {
        match request.protocol {
            TransportProtocol::Http => self.handle_http(request).await,
            TransportProtocol::WebSocket => self.handle_websocket(request).await,
            TransportProtocol::WebRtc => self.handle_webrtc(request).await,
        }
    }

    async fn handle_http(&self, request: ApiRequest) -> Result<ApiResponse, String> {
        let base_url = match request.provider.as_str() {
            "openai" => "https://api.openai.com/v1",
            "anthropic" => "https://api.anthropic.com/v1",
            "huggingface" => "https://api-inference.huggingface.co",
            _ => return Err(format!("Unknown provider: {}", request.provider)),
        };

        let url = format!("{}/{}", base_url, request.endpoint);
        log::info!("HTTP API call to {}: {}", request.provider, url);

        let opts = RequestInit::new();
        opts.set_method(&request.method);
        opts.set_mode(RequestMode::Cors);

        if !request.body.is_empty() {
            let body_js = unsafe { js_sys::Uint8Array::view(&request.body) };
            opts.set_body(&body_js);
        }

        let request_js = Request::new_with_str_and_init(&url, &opts)
            .map_err(|e| format!("Failed to create request: {:?}", e))?;

        for (key, value) in &request.headers {
            request_js
                .headers()
                .set(key, value)
                .map_err(|e| format!("Failed to set header {}: {:?}", key, e))?;
        }

        let window = web_sys::window().ok_or("No window found")?;
        let resp_value = JsFuture::from(window.fetch_with_request(&request_js))
            .await
            .map_err(|e| format!("Fetch failed: {:?}", e))?;

        let resp: Response = resp_value.dyn_into().unwrap();

        let body_value = JsFuture::from(resp.array_buffer().map_err(|e| format!("{:?}", e))?)
            .await
            .map_err(|e| format!("Buffer failed: {:?}", e))?;

        let body_array = js_sys::Uint8Array::new(&body_value);
        let mut body = vec![0; body_array.length() as usize];
        body_array.copy_to(&mut body);

        Ok(ApiResponse {
            status: resp.status(),
            headers: HashMap::new(), // TODO: Extract headers from resp
            body,
        })
    }

    async fn handle_websocket(&self, request: ApiRequest) -> Result<ApiResponse, String> {
        use gloo_net::websocket::futures::WebSocket;

        log::info!(
            "WebSocket API call to {}: {}",
            request.provider,
            request.endpoint
        );

        let url = request.endpoint; // Assume full URL for now
        let _ws =
            WebSocket::open(&url).map_err(|e| format!("Failed to open WebSocket: {:?}", e))?;

        // For UniProxy execute call, we might just return "connected"
        // In a real pattern, we would store this connection in the proxy
        Ok(ApiResponse {
            status: 101, // Switching Protocols
            headers: HashMap::new(),
            body: b"{\"status\": \"connected\"}".to_vec(),
        })
    }

    async fn handle_webrtc(&self, request: ApiRequest) -> Result<ApiResponse, String> {
        log::info!(
            "WebRTC API call to {}: {}",
            request.provider,
            request.endpoint
        );

        let config = RtcConfiguration::new();
        let ice_servers = js_sys::Array::new();
        let server = RtcIceServer::new();
        server.set_urls(&"stun:stun.l.google.com:19302".into());
        ice_servers.push(&server);
        config.set_ice_servers(&ice_servers);

        let _pc = RtcPeerConnection::new_with_configuration(&config)
            .map_err(|e| format!("Failed to create RtcPeerConnection: {:?}", e))?;

        // Signaling logic would go here:
        // 1. Create Data Channel
        // 2. Create Offer
        // 3. Exchange SDP via signaling server (WebSocket)

        Ok(ApiResponse {
            status: 200,
            headers: HashMap::new(),
            body: b"{\"status\": \"webrtc_initiated\"}".to_vec(),
        })
    }

    /// Convenience method for OpenAI chat completion
    pub async fn openai_chat(
        &self,
        api_key: &str,
        model: &str,
        messages: Vec<(String, String)>, // (role, content)
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

        if response.status != 200 {
            return Err(format!("API error: {}", response.status));
        }

        // Parse response
        let response_json: serde_json::Value =
            serde_json::from_slice(&response.body).map_err(|e| e.to_string())?;

        let content = response_json["choices"][0]["message"]["content"]
            .as_str()
            .ok_or("Invalid response format")?;

        Ok(content.to_string())
    }

    /// Convenience method for embeddings
    pub async fn openai_embeddings(
        &self,
        api_key: &str,
        model: &str,
        input: Vec<String>,
    ) -> Result<Vec<Vec<f32>>, String> {
        let mut headers = HashMap::new();
        headers.insert("Authorization".to_string(), format!("Bearer {}", api_key));
        headers.insert("Content-Type".to_string(), "application/json".to_string());

        let body = serde_json::json!({
            "model": model,
            "input": input
        });

        let request = ApiRequest {
            protocol: TransportProtocol::Http,
            pattern: RequestPattern::Unary,
            provider: "openai".to_string(),
            endpoint: "embeddings".to_string(),
            method: "POST".to_string(),
            headers,
            body: serde_json::to_vec(&body).map_err(|e| e.to_string())?,
        };

        let response = self.call_api(request).await?;

        if response.status != 200 {
            return Err(format!("API error: {}", response.status));
        }

        // Parse response
        let response_json: serde_json::Value =
            serde_json::from_slice(&response.body).map_err(|e| e.to_string())?;

        let embeddings: Vec<Vec<f32>> = response_json["data"]
            .as_array()
            .ok_or("Invalid response format")?
            .iter()
            .map(|item| {
                item["embedding"]
                    .as_array()
                    .unwrap()
                    .iter()
                    .map(|v| v.as_f64().unwrap() as f32)
                    .collect()
            })
            .collect();

        Ok(embeddings)
    }

    /// AI Shader Assistant: Generate WGSL code from a natural language prompt
    pub async fn ai_generate_shader(&self, api_key: &str, prompt: &str) -> Result<String, String> {
        let system_prompt = "You are an expert WGSL shader developer. Generate a highly optimized, single-file WGSL compute shader based on the user's request. Include the main entry point. ONLY return the code, no markdown blocks.";
        self.openai_chat(
            api_key,
            "gpt-4",
            vec![
                ("system".to_string(), system_prompt.to_string()),
                ("user".to_string(), prompt.to_string()),
            ],
        )
        .await
    }
}

/// Trait for decentralized shader discovery and registration
/// Note: Currently implemented but not actively called - reserved for future P2P shader registry
#[allow(dead_code)]
#[async_trait(?Send)]
pub trait ShaderFetcher {
    async fn fetch_shader(&self, url: &str) -> Result<String, String>;
    async fn register_shader(&self, manifest: ShaderManifest) -> Result<(), String>;
}

#[async_trait(?Send)]
impl ShaderFetcher for ApiProxy {
    async fn fetch_shader(&self, url: &str) -> Result<String, String> {
        let request = ApiRequest {
            protocol: TransportProtocol::Http,
            pattern: RequestPattern::Unary,
            provider: "custom".to_string(), // Hook for direct URLs
            endpoint: url.to_string(),
            method: "GET".to_string(),
            headers: HashMap::new(),
            body: vec![],
        };

        // Note: handle_http needs adjustment to handle relative/direct URLs
        let response = self.handle_http(request).await?;
        String::from_utf8(response.body).map_err(|e| e.to_string())
    }

    async fn register_shader(&self, _manifest: ShaderManifest) -> Result<(), String> {
        // Future: Register with a decentralized registry (e.g. KV store via API)
        Ok(())
    }
}

#[async_trait(?Send)]
impl UnitProxy for ApiProxy {
    fn service_name(&self) -> &str {
        "compute"
    }

    fn name(&self) -> &str {
        "api_proxy"
    }

    fn actions(&self) -> Vec<&str> {
        vec!["call", "chat", "embeddings"]
    }

    fn resource_limits(&self) -> ResourceLimits {
        ResourceLimits::default()
    }

    async fn execute(
        &self,
        method: &str,
        _input: &[u8],
        params: &[u8],
    ) -> Result<Vec<u8>, ComputeError> {
        match method {
            "call" => {
                let req: ApiRequest = serde_json::from_slice(params)
                    .map_err(|e| ComputeError::InvalidParams(e.to_string()))?;
                let resp = self
                    .call_api(req)
                    .await
                    .map_err(|e| ComputeError::ExecutionFailed(e))?;
                serde_json::to_vec(&resp).map_err(|e| ComputeError::ExecutionFailed(e.to_string()))
            }
            "chat" => {
                let params: serde_json::Value = serde_json::from_slice(params)
                    .map_err(|e| ComputeError::InvalidParams(e.to_string()))?;
                let api_key = params["api_key"]
                    .as_str()
                    .ok_or_else(|| ComputeError::InvalidParams("Missing api_key".to_string()))?;
                let model = params["model"].as_str().unwrap_or("gpt-4");
                let messages_raw = params["messages"]
                    .as_array()
                    .ok_or_else(|| ComputeError::InvalidParams("Missing messages".to_string()))?;

                let mut messages = Vec::new();
                for m in messages_raw {
                    let role = m["role"].as_str().unwrap_or("user").to_string();
                    let content = m["content"].as_str().unwrap_or("").to_string();
                    messages.push((role, content));
                }

                let resp = self
                    .openai_chat(api_key, model, messages)
                    .await
                    .map_err(|e| ComputeError::ExecutionFailed(e))?;
                Ok(resp.into_bytes())
            }
            "embeddings" => {
                let params: serde_json::Value = serde_json::from_slice(params)
                    .map_err(|e| ComputeError::InvalidParams(e.to_string()))?;
                let api_key = params["api_key"]
                    .as_str()
                    .ok_or_else(|| ComputeError::InvalidParams("Missing api_key".to_string()))?;
                let model = params["model"].as_str().unwrap_or("text-embedding-3-small");
                let input_raw = params["input"]
                    .as_array()
                    .ok_or_else(|| ComputeError::InvalidParams("Missing input".to_string()))?;

                let input: Vec<String> = input_raw
                    .iter()
                    .map(|v| v.as_str().unwrap_or("").to_string())
                    .collect();

                let resp = self
                    .openai_embeddings(api_key, model, input)
                    .await
                    .map_err(|e| ComputeError::ExecutionFailed(e))?;
                serde_json::to_vec(&resp).map_err(|e| ComputeError::ExecutionFailed(e.to_string()))
            }
            "generate_shader" => {
                let params: serde_json::Value = serde_json::from_slice(params)
                    .map_err(|e| ComputeError::InvalidParams(e.to_string()))?;
                let api_key = params["api_key"]
                    .as_str()
                    .ok_or_else(|| ComputeError::InvalidParams("Missing api_key".to_string()))?;
                let prompt = params["prompt"]
                    .as_str()
                    .ok_or_else(|| ComputeError::InvalidParams("Missing prompt".to_string()))?;

                let shader = self
                    .ai_generate_shader(api_key, prompt)
                    .await
                    .map_err(|e| ComputeError::ExecutionFailed(e))?;
                Ok(shader.into_bytes())
            }
            _ => Err(ComputeError::UnknownMethod {
                library: "api_proxy".to_string(),
                method: method.to_string(),
            }),
        }
    }
}

impl Default for ApiProxy {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_api_proxy_creation() {
        let _proxy = ApiProxy::new();
        assert!(true); // Placeholder
    }

    #[tokio::test]
    async fn test_api_request_structure() {
        let request = ApiRequest {
            protocol: TransportProtocol::Http,
            pattern: RequestPattern::Unary,
            provider: "openai".to_string(),
            endpoint: "chat/completions".to_string(),
            method: "POST".to_string(),
            headers: HashMap::new(),
            body: vec![],
        };

        assert_eq!(request.provider, "openai");
        assert_eq!(request.endpoint, "chat/completions");
    }
}
