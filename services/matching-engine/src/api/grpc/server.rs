use std::pin::Pin;
use std::sync::Arc;

use tokio::sync::broadcast;
use tokio_stream::wrappers::BroadcastStream;
use tokio_stream::Stream;
use tonic::{Request, Response, Status};
use uuid::Uuid;

use crate::domain::trade::Trade;
use crate::engine::amm::LmsrAmm;
use crate::market::manager::MarketManager;

// Include generated protobuf code
pub mod pb {
    tonic::include_proto!("matching");
}

use pb::matching_service_server::MatchingService;
use pb::{
    GetMarketOddsRequest, GetOrderBookRequest, MarketOddsResponse, OrderBookResponse,
    StreamTradesRequest, TradeEvent,
};

pub struct MatchingGrpcServer {
    market_manager: Arc<MarketManager>,
}

impl MatchingGrpcServer {
    pub fn new(market_manager: Arc<MarketManager>) -> Self {
        Self { market_manager }
    }
}

#[tonic::async_trait]
impl MatchingService for MatchingGrpcServer {
    async fn get_order_book(
        &self,
        req: Request<GetOrderBookRequest>,
    ) -> Result<Response<OrderBookResponse>, Status> {
        let market_id = parse_uuid(&req.get_ref().market_id)?;

        match self.market_manager.get_snapshot(market_id).await {
            Some(snap) => Ok(Response::new(OrderBookResponse {
                market_id: market_id.to_string(),
                best_bid_price: snap.best_bid.unwrap_or(0),
                best_ask_price: snap.best_ask.unwrap_or(0),
                bid_depth: snap.bid_depth as u64,
                ask_depth: snap.ask_depth as u64,
                seq_no: snap.seq,
            })),
            None => Err(Status::not_found(format!("market {} not active", market_id))),
        }
    }

    async fn get_market_odds(
        &self,
        req: Request<GetMarketOddsRequest>,
    ) -> Result<Response<MarketOddsResponse>, Status> {
        let market_id = parse_uuid(&req.get_ref().market_id)?;

        // For now: return default AMM odds if market not active yet.
        // Production: read from Redis cache (set by market task after each fill).
        let amm = LmsrAmm::new(100.0, 2);
        let odds = amm.current_odds();
        let prices: Vec<u64> = (0..odds.len()).map(|i| amm.price_minor(i)).collect();

        Ok(Response::new(MarketOddsResponse {
            market_id: market_id.to_string(),
            odds,
            prices_minor: prices,
        }))
    }

    type StreamTradesStream =
        Pin<Box<dyn Stream<Item = Result<TradeEvent, Status>> + Send + 'static>>;

    async fn stream_trades(
        &self,
        req: Request<StreamTradesRequest>,
    ) -> Result<Response<Self::StreamTradesStream>, Status> {
        let market_id = parse_uuid(&req.get_ref().market_id)?;
        let rx: broadcast::Receiver<Trade> = self.market_manager.subscribe_trades();

        let stream = BroadcastStream::new(rx)
            .filter_map(move |result| {
                let mid = market_id;
                async move {
                    match result {
                        Ok(trade) if trade.market_id == mid => Some(Ok(trade_to_event(&trade))),
                        Ok(_) => None, // different market — skip
                        Err(_) => None, // lagged receiver
                    }
                }
            });

        Ok(Response::new(Box::pin(stream)))
    }
}

fn parse_uuid(s: &str) -> Result<Uuid, Status> {
    Uuid::parse_str(s).map_err(|_| Status::invalid_argument(format!("invalid UUID: {}", s)))
}

fn trade_to_event(t: &Trade) -> TradeEvent {
    TradeEvent {
        trade_id: t.id.to_string(),
        market_id: t.market_id.to_string(),
        buyer_id: t.buyer_id.to_string(),
        seller_id: t.seller_id.map(|u| u.to_string()).unwrap_or_default(),
        price_minor: t.price_minor,
        quantity: t.quantity,
        outcome_index: t.outcome_index as u32,
        match_type: format!("{:?}", t.match_type).to_lowercase(),
        seq_no: t.seq_no,
        matched_at: t.matched_at.to_rfc3339(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::trade::MatchType;

    #[test]
    fn parse_uuid_valid() {
        let id = uuid::Uuid::new_v4();
        let result = parse_uuid(&id.to_string());
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), id);
    }

    #[test]
    fn parse_uuid_invalid() {
        let result: Result<Uuid, Status> = parse_uuid("not-a-uuid");
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code(), tonic::Code::InvalidArgument);
    }

    #[test]
    fn trade_to_event_clob() {
        let trade = Trade::new_clob(Uuid::new_v4(), Uuid::new_v4(), Uuid::new_v4(), 65, 150, 0, 42);
        let event = trade_to_event(&trade);
        assert_eq!(event.price_minor, 65);
        assert_eq!(event.quantity, 150);
        assert_eq!(event.match_type, "clob");
        assert_eq!(event.seq_no, 42);
        assert!(!event.seller_id.is_empty());
    }

    #[test]
    fn trade_to_event_amm() {
        let trade = Trade::new_amm(Uuid::new_v4(), Uuid::new_v4(), 50, 200, 1, 99);
        let event = trade_to_event(&trade);
        assert_eq!(event.price_minor, 50);
        assert_eq!(event.quantity, 200);
        assert_eq!(event.match_type, "amm");
        assert_eq!(event.seq_no, 99);
        assert!(event.seller_id.is_empty());
    }

    #[test]
    fn trade_to_event_rfc3339_timestamp() {
        let trade = Trade::new_clob(Uuid::new_v4(), Uuid::new_v4(), Uuid::new_v4(), 60, 100, 0, 1);
        let event = trade_to_event(&trade);
        // RFC 3339 format check: should contain 'T' and 'Z' or timezone offset
        assert!(event.matched_at.contains('T'));
        assert!(event.matched_at.contains('+') || event.matched_at.contains('Z'));
    }
}
