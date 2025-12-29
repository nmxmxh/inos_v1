use crate::jobs::{
    inference::InferenceJob, layers::LayersJob, tensor::TensorJob, training::TrainingJob,
};
use std::cell::RefCell;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum MLError {
    #[error("Tensor operation failed: {0}")]
    TensorError(#[from] candle_core::Error),

    #[error("Model loading failed: {0}")]
    ModelLoadError(String),

    #[error("Inference failed: {0}")]
    InferenceError(String),

    #[error("Invalid parameters: {0}")]
    InvalidParams(String),

    #[error("JSON error: {0}")]
    JsonError(#[from] serde_json::Error),

    #[error("Unknown method: {library}.{method}")]
    UnknownMethod { library: String, method: String },

    #[error("Device error: {0}")]
    DeviceError(String),

    #[error("Delegate execution to WebGPU")]
    Delegate(Vec<u8>),
}

impl From<String> for MLError {
    fn from(s: String) -> Self {
        MLError::TensorError(candle_core::Error::Msg(s))
    }
}

pub type Result<T> = std::result::Result<T, MLError>;

/// Production ML Engine - orchestrates all ML operations
///
/// Provides method discovery for all 45 operations:
/// - tensor_* → TensorJob (15 ops)
/// - linear, conv2d, etc → LayersJob (11 ops)
/// - mse_loss, adam_step, etc → TrainingJob (14 ops)
/// - run_llm, quantize_*, etc → InferenceJob (9 ops)
///
/// Note: Actual execution is handled by the kernel via Cap'n Proto.
/// This engine provides method discovery and validation.
pub struct MLEngine {
    tensor_job: RefCell<TensorJob>,
    layers_job: RefCell<LayersJob>,
    training_job: RefCell<TrainingJob>,
    inference_job: RefCell<InferenceJob>,
}

impl MLEngine {
    pub fn new() -> Result<Self> {
        Ok(Self {
            tensor_job: RefCell::new(TensorJob::new()?),
            layers_job: RefCell::new(LayersJob::new()?),
            training_job: RefCell::new(TrainingJob::new()?),
            inference_job: RefCell::new(InferenceJob::new()?),
        })
    }

    /// Get all available methods (45 operations)
    pub fn methods(&self) -> Vec<&'static str> {
        let mut all_methods = Vec::new();

        // Collect from all jobs
        all_methods.extend(self.tensor_job.borrow().methods());
        all_methods.extend(self.layers_job.borrow().methods());
        all_methods.extend(self.training_job.borrow().methods());
        all_methods.extend(self.inference_job.borrow().methods());

        all_methods
    }

    /// Execute ML operation (Discovery + Dispatch)
    pub fn execute(
        &self,
        library: &str,
        method: &str,
        input: &[u8],
        params: &str,
    ) -> Result<Vec<u8>> {
        match library {
            "tensor" => self.tensor_job.borrow_mut().execute(method, input, params),
            "layers" => self.layers_job.borrow_mut().execute(method, input, params),
            "training" => self
                .training_job
                .borrow_mut()
                .execute(method, input, params),
            "inference" => self
                .inference_job
                .borrow_mut()
                .execute(method, input, params),
            _ => Err(MLError::UnknownMethod {
                library: library.to_string(),
                method: method.to_string(),
            }),
        }
    }
}
