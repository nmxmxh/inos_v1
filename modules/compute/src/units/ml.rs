use crate::engine::{ComputeError, ResourceLimits, UnitProxy};
use async_trait::async_trait;
use ml::MLEngine;

pub struct MLUnit {
    engine: MLEngine,
}

impl MLUnit {
    pub fn new() -> Result<Self, Box<dyn std::error::Error>> {
        let engine =
            MLEngine::new().map_err(|e| format!("Failed to initialize ML engine: {}", e))?;
        Ok(Self { engine })
    }
}

#[async_trait(?Send)]
impl UnitProxy for MLUnit {
    fn service_name(&self) -> &str {
        "ml"
    }

    fn name(&self) -> &str {
        "ml"
    }

    fn actions(&self) -> Vec<&str> {
        self.engine.methods()
    }

    fn resource_limits(&self) -> ResourceLimits {
        ResourceLimits {
            max_input_size: 100 * 1024 * 1024,  // 100MB for models/tensors
            max_output_size: 100 * 1024 * 1024, // 100MB
            max_memory_pages: 16384,            // 1GB (ML needs memory)
            timeout_ms: 60000,                  // 60s
            max_fuel: 500_000_000_000,          // 500B instructions
        }
    }

    async fn execute(
        &self,
        action: &str,
        input: &[u8],
        params: &[u8],
    ) -> Result<Vec<u8>, ComputeError> {
        // Dissect action to determine library
        let (library, method) = if action.starts_with("tensor_") {
            ("tensor", action)
        } else if action.contains("loss") || action.contains("step") || action == "clip_gradients" {
            if action.contains("step") && !action.contains("adam") && !action.contains("sgd") {
                ("layers", action) // Catch-all for other steps if any
            } else {
                ("training", action)
            }
        } else if [
            "linear",
            "conv2d",
            "max_pool2d",
            "avg_pool2d",
            "layer_norm",
            "batch_norm",
            "group_norm",
            "attention",
            "multi_head_attention",
            "cross_attention",
            "rope",
        ]
        .contains(&action)
        {
            ("layers", action)
        } else {
            ("inference", action)
        };

        // Temporary: Convert to string for MLEngine (generic path)
        let params_str = std::str::from_utf8(params)
            .map_err(|e| ComputeError::InvalidParams(format!("Params not valid UTF-8: {}", e)))?;

        self.engine
            .execute(library, method, input, params_str)
            .map_err(|e| ComputeError::ExecutionFailed(format!("ML execution failed: {}", e)))
    }
}
