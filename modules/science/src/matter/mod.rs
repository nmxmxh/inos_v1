pub mod atomic;
pub mod continuum;
pub mod kinetic;

pub use crate::types::{ScienceError, ScienceResult};

pub trait ScienceProxy {
    fn name(&self) -> &'static str;
    fn methods(&self) -> Vec<&'static str>;
    fn execute(&mut self, method: &str, input: &[u8], params: &[u8]) -> ScienceResult<Vec<u8>>;

    fn validate_spot(
        &mut self,
        _method: &str,
        _input: &[u8],
        _params: &[u8],
        _spot_seed: &[u8],
    ) -> ScienceResult<Vec<u8>> {
        // Default implementation returns empty result
        Ok(Vec::new())
    }
}
