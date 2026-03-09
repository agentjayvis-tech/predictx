use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

/// Which side of the market the order is on.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "UPPERCASE")]
pub enum OrderSide {
    Yes,
    No,
}

impl OrderSide {
    /// Returns the complementary side (YES ↔ NO).
    pub fn opposite(&self) -> Self {
        match self {
            Self::Yes => Self::No,
            Self::No => Self::Yes,
        }
    }
}

/// Whether the order is a limit or market order.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum OrderType {
    Limit,
    Market,
}

/// Lifecycle state of an order.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum OrderStatus {
    Pending,
    PartiallyFilled,
    Filled,
    Cancelled,
}

/// An order submitted to the matching engine.
/// Prices are in minor units (e.g. paise/cents × 100), range 1–99.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Order {
    pub id: Uuid,
    pub market_id: Uuid,
    pub user_id: Uuid,
    pub side: OrderSide,
    pub order_type: OrderType,
    /// Price in minor units (1–99 for YES; complementary for NO). 0 = market order.
    pub price_minor: u64,
    /// Total quantity in shares.
    pub quantity: u64,
    /// Remaining unfilled quantity.
    pub remaining: u64,
    /// Which outcome this order is for (0-indexed).
    pub outcome_index: usize,
    /// Monotonically increasing per-market sequence number (assigned by engine).
    pub seq_no: u64,
    /// Unique key to prevent duplicate processing.
    pub idempotency_key: String,
    pub placed_at: DateTime<Utc>,
}

impl Order {
    pub fn is_filled(&self) -> bool {
        self.remaining == 0
    }

    pub fn fill(&mut self, qty: u64) {
        self.remaining = self.remaining.saturating_sub(qty);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn opposite_side() {
        assert_eq!(OrderSide::Yes.opposite(), OrderSide::No);
        assert_eq!(OrderSide::No.opposite(), OrderSide::Yes);
    }

    #[test]
    fn order_fill() {
        let mut order = Order {
            id: Uuid::new_v4(),
            market_id: Uuid::new_v4(),
            user_id: Uuid::new_v4(),
            side: OrderSide::Yes,
            order_type: OrderType::Limit,
            price_minor: 60,
            quantity: 100,
            remaining: 100,
            outcome_index: 0,
            seq_no: 0,
            idempotency_key: "test".into(),
            placed_at: Utc::now(),
        };
        order.fill(40);
        assert_eq!(order.remaining, 60);
        assert!(!order.is_filled());
        order.fill(60);
        assert!(order.is_filled());
    }
}
