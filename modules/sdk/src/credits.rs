use web_sys::Performance;

pub struct CostTracker {
    perf: Performance,
    start_time: f64,
}

impl CostTracker {
    pub fn new() -> Option<Self> {
        let window = web_sys::window()?;
        let perf = window.performance()?;
        Some(Self {
            perf,
            start_time: 0.0,
        })
    }

    pub fn start(&mut self) {
        self.start_time = self.perf.now();
    }

    pub fn stop(&self) -> f64 {
        // Returns ms duration
        self.perf.now() - self.start_time
    }
}

pub struct BudgetVerifier {
    budget: u64,
    consumed: u64,
}

impl BudgetVerifier {
    pub fn new(budget: u64) -> Self {
        Self {
            budget,
            consumed: 0,
        }
    }

    pub fn consume(&mut self, amount: u64) -> Result<(), &'static str> {
        self.consumed += amount;
        if self.consumed > self.budget {
            Err("OutOfCredits")
        } else {
            Ok(())
        }
    }
}
