use async_trait::async_trait;
use std::collections::HashMap;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum ExecutorError {
    #[error("Unknown job type: {0}")]
    UnknownJobType(String),

    #[error("Input validation failed: {0}")]
    InvalidInput(String),

    #[error("Output verification failed: {0}")]
    InvalidOutput(String),

    #[error("Insufficient budget: estimated {estimated}, available {available}")]
    InsufficientBudget { estimated: u64, available: u64 },

    #[error("Execution failed: {0}")]
    ExecutionFailed(String),
}

pub type Result<T> = std::result::Result<T, ExecutorError>;

/// JobType enum matching Cap'n Proto schema
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum JobType {
    ImageProcess,
    VideoEncode,
    CryptoOperation,
    CustomShader,
    MLInference,
    PhysicsStep,
    AudioProcess,
    Render3D,
    DataAnalysis,
}

impl JobType {
    pub fn from_u16(value: u16) -> Option<Self> {
        match value {
            0 => Some(JobType::ImageProcess),
            1 => Some(JobType::VideoEncode),
            2 => Some(JobType::CryptoOperation),
            3 => Some(JobType::CustomShader),
            4 => Some(JobType::MLInference),
            5 => Some(JobType::PhysicsStep),
            6 => Some(JobType::AudioProcess),
            7 => Some(JobType::Render3D),
            8 => Some(JobType::DataAnalysis),
            _ => None,
        }
    }
}

/// Generic trait for all compute jobs
/// MUST be deterministic (same input = same output)
#[async_trait(?Send)]
pub trait ComputeJob {
    /// Execute the job (must be deterministic!)
    async fn execute(&self, input: &[u8]) -> Result<Vec<u8>>;

    /// Estimate cost in credits (for budget verification)
    fn estimate_cost(&self, input_size: usize) -> u64;

    /// Validate input before execution (prevent exploits)
    fn validate_input(&self, _input: &[u8]) -> Result<()> {
        Ok(()) // Default: no validation
    }

    /// Verify output after execution (prevent garbage)
    fn verify_output(&self, _output: &[u8]) -> Result<()> {
        Ok(()) // Default: no verification
    }

    /// Job type identifier
    fn job_type(&self) -> JobType;
}

/// Job registry (plugin system)
pub struct JobRegistry {
    handlers: HashMap<JobType, Box<dyn ComputeJob>>,
}

impl JobRegistry {
    pub fn new() -> Self {
        Self {
            handlers: HashMap::new(),
        }
    }

    /// Register a job handler
    pub fn register(&mut self, job: Box<dyn ComputeJob>) {
        self.handlers.insert(job.job_type(), job);
    }

    /// Execute a job with validation and budget verification
    pub async fn execute(&self, job_type: JobType, input: &[u8], budget: u64) -> Result<Vec<u8>> {
        if input.is_empty() {
            return Err(ExecutorError::InvalidInput(
                "Input data cannot be empty".to_string(),
            ));
        }
        // 1. Get handler
        let handler = self
            .handlers
            .get(&job_type)
            .ok_or_else(|| ExecutorError::UnknownJobType(format!("{:?}", job_type)))?;

        // 2. Estimate cost
        let estimated_cost = handler.estimate_cost(input.len());

        // 3. Verify budget
        if estimated_cost > budget {
            return Err(ExecutorError::InsufficientBudget {
                estimated: estimated_cost,
                available: budget,
            });
        }

        // 4. Validate input
        handler.validate_input(input)?;

        // 5. Execute job
        let output = handler
            .execute(input)
            .await
            .map_err(|e| ExecutorError::ExecutionFailed(e.to_string()))?;

        if output.is_empty() {
            return Err(ExecutorError::InvalidOutput(
                "Job produced empty output".to_string(),
            ));
        }

        // 6. Verify output
        handler.verify_output(&output)?;

        Ok(output)
    }
}

impl Default for JobRegistry {
    fn default() -> Self {
        Self::new()
    }
}
