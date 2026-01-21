use crate::executor::{ComputeJob, JobType, Result};
use async_trait::async_trait;

/// Stub job handler for unimplemented job types
pub struct StubJob {
    job_type: JobType,
}

impl StubJob {
    pub fn new(job_type: JobType) -> Self {
        Self { job_type }
    }
}

#[async_trait]
impl ComputeJob for StubJob {
    async fn execute(&self, input: &[u8]) -> Result<Vec<u8>> {
        let message = format!(
            "Job type {:?} not yet implemented. Input size: {} bytes",
            self.job_type,
            input.len()
        );
        Ok(message.into_bytes())
    }

    fn estimate_cost(&self, _input_size: usize) -> u64 {
        1 // Minimal cost for stub
    }

    fn job_type(&self) -> JobType {
        self.job_type
    }
}
