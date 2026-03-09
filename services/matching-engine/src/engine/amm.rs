/// Logarithmic Market Scoring Rule (LMSR) AMM.
///
/// C(q) = b * ln(Σ exp(q_i / b))
///
/// The instantaneous price (probability) for outcome i is:
///   p_i = exp(q_i / b) / Σ exp(q_j / b)
///
/// Cost to purchase Δq_i shares of outcome i:
///   cost = C(q after) - C(q before)
///
/// `b` controls liquidity depth: larger b = deeper liquidity, higher max loss.
#[derive(Debug, Clone)]
pub struct LmsrAmm {
    /// Liquidity parameter. Typical range: 50–500 depending on market size.
    b: f64,
    /// Cumulative shares sold per outcome.
    quantities: Vec<f64>,
}

impl LmsrAmm {
    pub fn new(b: f64, n_outcomes: usize) -> Self {
        assert!(b > 0.0, "liquidity parameter b must be positive");
        assert!(n_outcomes >= 2, "must have at least 2 outcomes");
        Self {
            b,
            quantities: vec![0.0; n_outcomes],
        }
    }

    /// Total cost function C(q).
    pub fn cost(&self) -> f64 {
        let max_q = self.quantities.iter().cloned().fold(f64::NEG_INFINITY, f64::max);
        // Numerically stable via log-sum-exp trick
        let lse = max_q
            + self
                .quantities
                .iter()
                .map(|&q| (q / self.b - max_q / self.b).exp())
                .sum::<f64>()
                .ln();
        self.b * lse
    }

    /// Current implied odds (probabilities) for each outcome. Sum = 1.0.
    pub fn current_odds(&self) -> Vec<f64> {
        let max_q = self.quantities.iter().cloned().fold(f64::NEG_INFINITY, f64::max);
        let exps: Vec<f64> = self.quantities.iter().map(|&q| ((q - max_q) / self.b).exp()).collect();
        let sum: f64 = exps.iter().sum();
        exps.iter().map(|e| e / sum).collect()
    }

    /// Current price in minor units (1–99) for outcome `i`.
    pub fn price_minor(&self, outcome: usize) -> u64 {
        let odds = self.current_odds();
        (odds[outcome] * 99.0).round().clamp(1.0, 99.0) as u64
    }

    /// Compute cost to purchase `delta` shares of outcome `outcome`.
    /// Returns (cost_minor: u64, new_odds: Vec<f64>).
    pub fn quote_buy(&self, outcome: usize, delta: f64) -> (f64, Vec<f64>) {
        let before = self.cost();
        let mut after_quantities = self.quantities.clone();
        after_quantities[outcome] += delta;
        let after_amm = LmsrAmm { b: self.b, quantities: after_quantities };
        let cost = after_amm.cost() - before;
        (cost, after_amm.current_odds())
    }

    /// Execute a buy of `delta` shares of `outcome`. Mutates internal state.
    /// Returns (average_price_minor, new_odds).
    pub fn fill_order(&mut self, outcome: usize, delta: f64) -> (u64, Vec<f64>) {
        let (cost, new_odds) = self.quote_buy(outcome, delta);
        self.quantities[outcome] += delta;
        // Average fill price per share in minor units
        let avg_price = if delta > 0.0 {
            ((cost / delta) * 99.0).round().clamp(1.0, 99.0) as u64
        } else {
            0
        };
        (avg_price, new_odds)
    }

    /// Can the AMM fill an order at or better than `limit_price_minor` for `outcome`?
    pub fn can_fill(&self, outcome: usize, limit_price_minor: u64) -> bool {
        let current = self.price_minor(outcome);
        current <= limit_price_minor
    }

    pub fn n_outcomes(&self) -> usize {
        self.quantities.len()
    }

    pub fn quantities(&self) -> &[f64] {
        &self.quantities
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn initial_odds_equal_for_binary() {
        let amm = LmsrAmm::new(100.0, 2);
        let odds = amm.current_odds();
        assert!((odds[0] - 0.5).abs() < 1e-9);
        assert!((odds[1] - 0.5).abs() < 1e-9);
    }

    #[test]
    fn odds_sum_to_one() {
        let mut amm = LmsrAmm::new(100.0, 3);
        amm.fill_order(0, 200.0);
        amm.fill_order(1, 50.0);
        let odds = amm.current_odds();
        let sum: f64 = odds.iter().sum();
        assert!((sum - 1.0).abs() < 1e-9);
    }

    #[test]
    fn buying_shifts_price_up() {
        let mut amm = LmsrAmm::new(100.0, 2);
        let p0 = amm.price_minor(0);
        amm.fill_order(0, 500.0);
        let p1 = amm.price_minor(0);
        assert!(p1 > p0, "buying YES shares should increase YES price");
    }

    #[test]
    fn cost_is_positive() {
        let amm = LmsrAmm::new(100.0, 2);
        let (cost, _) = amm.quote_buy(0, 100.0);
        assert!(cost > 0.0);
    }

    #[test]
    fn can_fill_at_50_initially() {
        let amm = LmsrAmm::new(100.0, 2);
        // Initial price is ~50; market order (price=99) should always fill
        assert!(amm.can_fill(0, 99));
    }
}
