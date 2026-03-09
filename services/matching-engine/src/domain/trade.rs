use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

/// How the trade was filled.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, sqlx::Type)]
#[serde(rename_all = "lowercase")]
#[sqlx(type_name = "match_type", rename_all = "lowercase")]
pub enum MatchType {
    /// Matched via Central Limit Order Book (two users).
    Clob,
    /// Filled via AMM liquidity (Logarithmic Market Scoring Rule).
    Amm,
}

/// A matched trade produced by the engine.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Trade {
    pub id: Uuid,
    pub market_id: Uuid,
    /// The YES-side participant.
    pub buyer_id: Uuid,
    /// The NO-side participant. None for AMM fills.
    pub seller_id: Option<Uuid>,
    /// Agreed price in minor units.
    pub price_minor: u64,
    /// Quantity of shares exchanged.
    pub quantity: u64,
    /// Outcome index this trade is for.
    pub outcome_index: usize,
    pub match_type: MatchType,
    /// Monotonically increasing per-market sequence number.
    pub seq_no: u64,
    pub matched_at: DateTime<Utc>,
}

impl Trade {
    pub fn new_clob(
        market_id: Uuid,
        buyer_id: Uuid,
        seller_id: Uuid,
        price_minor: u64,
        quantity: u64,
        outcome_index: usize,
        seq_no: u64,
    ) -> Self {
        Self {
            id: Uuid::new_v4(),
            market_id,
            buyer_id,
            seller_id: Some(seller_id),
            price_minor,
            quantity,
            outcome_index,
            match_type: MatchType::Clob,
            seq_no,
            matched_at: Utc::now(),
        }
    }

    pub fn new_amm(
        market_id: Uuid,
        buyer_id: Uuid,
        price_minor: u64,
        quantity: u64,
        outcome_index: usize,
        seq_no: u64,
    ) -> Self {
        Self {
            id: Uuid::new_v4(),
            market_id,
            buyer_id,
            seller_id: None,
            price_minor,
            quantity,
            outcome_index,
            match_type: MatchType::Amm,
            seq_no,
            matched_at: Utc::now(),
        }
    }
}
