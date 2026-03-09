use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::domain::order::{Order, OrderSide, OrderType};
use crate::domain::trade::{MatchType, Trade};

// ─── Inbound (orders.placed) ─────────────────────────────────────────────────

/// Event published by Order Service onto `orders.placed`.
#[derive(Debug, Clone, Deserialize)]
pub struct OrderPlacedEvent {
    pub order_id: Uuid,
    pub user_id: Uuid,
    pub market_id: Uuid,
    pub side: String,         // "YES" | "NO"
    pub order_type: String,   // "LIMIT" | "MARKET"
    pub price_minor: u64,
    pub quantity: u64,
    pub outcome_index: usize,
    pub idempotency_key: String,
    pub placed_at: DateTime<Utc>,
}

impl OrderPlacedEvent {
    pub fn into_order(self) -> Option<Order> {
        let side = match self.side.to_uppercase().as_str() {
            "YES" => OrderSide::Yes,
            "NO" => OrderSide::No,
            _ => return None,
        };
        let order_type = match self.order_type.to_uppercase().as_str() {
            "LIMIT" => OrderType::Limit,
            "MARKET" => OrderType::Market,
            _ => return None,
        };
        Some(Order {
            id: self.order_id,
            market_id: self.market_id,
            user_id: self.user_id,
            side,
            order_type,
            price_minor: self.price_minor,
            quantity: self.quantity,
            remaining: self.quantity,
            outcome_index: self.outcome_index,
            seq_no: 0, // assigned by engine
            idempotency_key: self.idempotency_key,
            placed_at: self.placed_at,
        })
    }
}

// ─── Outbound (trades.matched) ────────────────────────────────────────────────

/// Event published by Matching Engine onto `trades.matched`.
#[derive(Debug, Clone, Serialize)]
pub struct TradeMatchedEvent {
    pub event: &'static str,
    pub trade_id: Uuid,
    pub market_id: Uuid,
    pub buyer_id: Uuid,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub seller_id: Option<Uuid>,
    pub price_minor: u64,
    pub quantity: u64,
    pub outcome_index: usize,
    pub match_type: String,
    pub seq_no: u64,
    pub matched_at: DateTime<Utc>,
}

impl From<&Trade> for TradeMatchedEvent {
    fn from(t: &Trade) -> Self {
        Self {
            event: "trade_matched",
            trade_id: t.id,
            market_id: t.market_id,
            buyer_id: t.buyer_id,
            seller_id: t.seller_id,
            price_minor: t.price_minor,
            quantity: t.quantity,
            outcome_index: t.outcome_index,
            match_type: match t.match_type {
                MatchType::Clob => "clob".to_string(),
                MatchType::Amm => "amm".to_string(),
            },
            seq_no: t.seq_no,
            matched_at: t.matched_at,
        }
    }
}

// ─── Inbound (market.voided) ──────────────────────────────────────────────────

/// Event from Market Service / Resolution Service when a market is voided.
#[derive(Debug, Deserialize)]
pub struct MarketVoidedEvent {
    pub market_id: Uuid,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn order_placed_event_into_order_valid() {
        let event = OrderPlacedEvent {
            order_id: Uuid::new_v4(),
            user_id: Uuid::new_v4(),
            market_id: Uuid::new_v4(),
            side: "YES".to_string(),
            order_type: "LIMIT".to_string(),
            price_minor: 70,
            quantity: 100,
            outcome_index: 0,
            idempotency_key: "test-key".into(),
            placed_at: Utc::now(),
        };

        let order = event.into_order().expect("should convert");
        assert_eq!(order.side, OrderSide::Yes);
        assert_eq!(order.order_type, OrderType::Limit);
        assert_eq!(order.price_minor, 70);
        assert_eq!(order.quantity, 100);
        assert_eq!(order.remaining, 100);
    }

    #[test]
    fn order_placed_event_into_order_invalid_side() {
        let event = OrderPlacedEvent {
            order_id: Uuid::new_v4(),
            user_id: Uuid::new_v4(),
            market_id: Uuid::new_v4(),
            side: "INVALID".to_string(),
            order_type: "LIMIT".to_string(),
            price_minor: 70,
            quantity: 100,
            outcome_index: 0,
            idempotency_key: "test-key".into(),
            placed_at: Utc::now(),
        };

        assert!(event.into_order().is_none());
    }

    #[test]
    fn order_placed_event_into_order_market_order() {
        let event = OrderPlacedEvent {
            order_id: Uuid::new_v4(),
            user_id: Uuid::new_v4(),
            market_id: Uuid::new_v4(),
            side: "no".to_string(), // case-insensitive
            order_type: "market".to_string(),
            price_minor: 0,
            quantity: 50,
            outcome_index: 1,
            idempotency_key: "test-key".into(),
            placed_at: Utc::now(),
        };

        let order = event.into_order().expect("should convert");
        assert_eq!(order.side, OrderSide::No);
        assert_eq!(order.order_type, OrderType::Market);
    }

    #[test]
    fn trade_matched_event_from_clob_trade() {
        let trade = Trade::new_clob(
            Uuid::new_v4(),
            Uuid::new_v4(),
            Uuid::new_v4(),
            60,
            100,
            0,
            1,
        );

        let event = TradeMatchedEvent::from(&trade);
        assert_eq!(event.event, "trade_matched");
        assert_eq!(event.price_minor, 60);
        assert_eq!(event.quantity, 100);
        assert_eq!(event.match_type, "clob");
        assert!(event.seller_id.is_some());
    }

    #[test]
    fn trade_matched_event_from_amm_trade() {
        let trade = Trade::new_amm(Uuid::new_v4(), Uuid::new_v4(), 50, 200, 1, 5);

        let event = TradeMatchedEvent::from(&trade);
        assert_eq!(event.event, "trade_matched");
        assert_eq!(event.price_minor, 50);
        assert_eq!(event.quantity, 200);
        assert_eq!(event.match_type, "amm");
        assert!(event.seller_id.is_none()); // AMM fills have no seller
    }

    #[test]
    fn order_placed_event_deserialize() {
        let json = r#"
        {
            "order_id": "550e8400-e29b-41d4-a716-446655440001",
            "user_id": "550e8400-e29b-41d4-a716-446655440002",
            "market_id": "550e8400-e29b-41d4-a716-446655440003",
            "side": "YES",
            "order_type": "LIMIT",
            "price_minor": 75,
            "quantity": 250,
            "outcome_index": 0,
            "idempotency_key": "idem-123",
            "placed_at": "2024-01-15T12:00:00Z"
        }
        "#;

        let event: OrderPlacedEvent = serde_json::from_str(json).expect("should deserialize");
        assert_eq!(event.side, "YES");
        assert_eq!(event.price_minor, 75);
        assert_eq!(event.quantity, 250);
    }

    #[test]
    fn trade_matched_event_serialize() {
        let trade = Trade::new_clob(Uuid::new_v4(), Uuid::new_v4(), Uuid::new_v4(), 65, 150, 0, 10);
        let event = TradeMatchedEvent::from(&trade);
        let json = serde_json::to_string(&event).expect("should serialize");
        assert!(json.contains("\"event\":\"trade_matched\""));
        assert!(json.contains("\"match_type\":\"clob\""));
    }

    #[test]
    fn market_voided_event_deserialize() {
        let market_id = Uuid::new_v4();
        let json = format!(r#"{{"market_id": "{}"}}"#, market_id);
        let event: MarketVoidedEvent = serde_json::from_str(&json).expect("should deserialize");
        assert_eq!(event.market_id, market_id);
    }
}
