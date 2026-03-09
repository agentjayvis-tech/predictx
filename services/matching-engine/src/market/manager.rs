use std::sync::Arc;

use dashmap::DashMap;
use tokio::sync::{broadcast, mpsc};
use tracing::{error, info, warn};
use uuid::Uuid;

use crate::domain::order::Order;
use crate::domain::trade::Trade;
use crate::engine::amm::LmsrAmm;
use crate::engine::matching::process_order;
use crate::engine::order_book::{OrderBook, OrderBookSnapshot};
use crate::events::publisher::Publisher;
use crate::metrics;
use crate::store::trade_repo::TradeRepo;
use crate::store::wal::Wal;

const CHANNEL_CAPACITY: usize = 8_192;
const TRADE_BROADCAST_CAPACITY: usize = 1_024;

/// Commands sent to a per-market task.
enum MarketCmd {
    Order(Order),
    Cancel(Uuid),
    /// Void: cancel all resting orders and shut down the task.
    Void,
    Snapshot(tokio::sync::oneshot::Sender<OrderBookSnapshot>),
    Shutdown,
}

/// Manages one Tokio task per active market.
/// Each task owns its OrderBook and LMSR AMM — no shared state on the hot path.
pub struct MarketManager {
    /// market_id → command sender
    markets: Arc<DashMap<Uuid, mpsc::Sender<MarketCmd>>>,
    /// Broadcast channel for trade events (gRPC streaming).
    trade_tx: broadcast::Sender<Trade>,
    publisher: Arc<Publisher>,
    trade_repo: Arc<TradeRepo>,
    wal: Arc<Wal>,
    amm_b: f64,
    amm_default_outcomes: usize,
}

impl MarketManager {
    pub fn new(
        publisher: Arc<Publisher>,
        trade_repo: Arc<TradeRepo>,
        wal: Arc<Wal>,
        amm_b: f64,
        amm_default_outcomes: usize,
    ) -> Self {
        let (trade_tx, _) = broadcast::channel(TRADE_BROADCAST_CAPACITY);
        Self {
            markets: Arc::new(DashMap::new()),
            trade_tx,
            publisher,
            trade_repo,
            wal,
            amm_b,
            amm_default_outcomes,
        }
    }

    /// Subscribe to the trade broadcast stream (for gRPC StreamTrades).
    pub fn subscribe_trades(&self) -> broadcast::Receiver<Trade> {
        self.trade_tx.subscribe()
    }

    /// Return a snapshot of the order book for a market (for gRPC GetOrderBook).
    pub async fn get_snapshot(&self, market_id: Uuid) -> Option<OrderBookSnapshot> {
        let tx = self.markets.get(&market_id)?.clone();
        let (resp_tx, resp_rx) = tokio::sync::oneshot::channel();
        tx.send(MarketCmd::Snapshot(resp_tx)).await.ok()?;
        resp_rx.await.ok()
    }

    /// Submit an order to the appropriate market task.
    /// Spawns the task if the market is not yet known.
    pub async fn submit_order(&self, order: Order) {
        let market_id = order.market_id;
        let tx = self.ensure_market(market_id);
        if let Err(e) = tx.send(MarketCmd::Order(order)).await {
            error!(market_id = %market_id, error = %e, "failed to send order to market task");
        }
    }

    /// Cancel an order by ID within a market.
    pub async fn cancel_order(&self, market_id: Uuid, order_id: Uuid) {
        if let Some(tx) = self.markets.get(&market_id) {
            let _ = tx.send(MarketCmd::Cancel(order_id)).await;
        }
    }

    /// Void a market: cancel all resting orders and shut down its task.
    pub async fn void_market(&self, market_id: Uuid) {
        if let Some(tx) = self.markets.get(&market_id) {
            let _ = tx.send(MarketCmd::Void).await;
        }
    }

    /// Shut down all market tasks gracefully.
    pub async fn shutdown(&self) {
        for entry in self.markets.iter() {
            let _ = entry.value().send(MarketCmd::Shutdown).await;
        }
    }

    fn ensure_market(&self, market_id: Uuid) -> mpsc::Sender<MarketCmd> {
        if let Some(tx) = self.markets.get(&market_id) {
            return tx.clone();
        }
        // Double-checked insert
        let (tx, rx) = mpsc::channel(CHANNEL_CAPACITY);
        if let Some(existing) = self.markets.get(&market_id) {
            return existing.clone();
        }
        self.markets.insert(market_id, tx.clone());
        metrics::ACTIVE_MARKETS.inc();
        info!(market_id = %market_id, "spawning market task");

        let publisher = self.publisher.clone();
        let trade_repo = self.trade_repo.clone();
        let wal = self.wal.clone();
        let trade_tx = self.trade_tx.clone();
        let markets = self.markets.clone();
        let amm_b = self.amm_b;
        let amm_default_outcomes = self.amm_default_outcomes;

        tokio::spawn(async move {
            run_market_task(
                market_id,
                rx,
                publisher,
                trade_repo,
                wal,
                trade_tx,
                markets,
                amm_b,
                amm_default_outcomes,
            )
            .await;
        });

        tx
    }
}

/// The inner loop of a per-market task.
/// Single-writer: only this task mutates the book and AMM — no locks on the hot path.
async fn run_market_task(
    market_id: Uuid,
    mut rx: mpsc::Receiver<MarketCmd>,
    publisher: Arc<Publisher>,
    trade_repo: Arc<TradeRepo>,
    wal: Arc<Wal>,
    trade_tx: broadcast::Sender<Trade>,
    markets: Arc<DashMap<Uuid, mpsc::Sender<MarketCmd>>>,
    amm_b: f64,
    amm_default_outcomes: usize,
) {
    let mut book = OrderBook::new();
    let mut amm = LmsrAmm::new(amm_b, amm_default_outcomes);

    while let Some(cmd) = rx.recv().await {
        match cmd {
            MarketCmd::Order(order) => {
                let start = std::time::Instant::now();
                let trades = process_order(&mut book, &mut amm, order);
                let elapsed_us = start.elapsed().as_micros() as f64 / 1000.0; // ms
                metrics::MATCH_LATENCY_MS.observe(elapsed_us);

                for trade in &trades {
                    metrics::ORDERS_PROCESSED
                        .with_label_values(&[format!("{:?}", trade.match_type).to_lowercase().as_str()])
                        .inc();

                    // Persist to WAL (fast, local)
                    if let Err(e) = wal.append(trade) {
                        error!(trade_id = %trade.id, error = %e, "WAL append failed");
                    }

                    // Persist to PostgreSQL (async, non-blocking)
                    let repo = trade_repo.clone();
                    let t = trade.clone();
                    tokio::spawn(async move {
                        if let Err(e) = repo.insert(&t).await {
                            error!(trade_id = %t.id, error = %e, "DB insert failed");
                        }
                    });

                    // Publish to Kafka (async, non-blocking to not stall match loop)
                    let pub_clone = publisher.clone();
                    let t = trade.clone();
                    tokio::spawn(async move {
                        if let Err(e) = pub_clone.publish_trade(&t).await {
                            warn!(trade_id = %t.id, error = %e, "Kafka publish failed");
                        } else {
                            metrics::TRADES_PUBLISHED.inc();
                        }
                    });

                    // Broadcast to gRPC streaming subscribers
                    let _ = trade_tx.send(trade.clone());
                }
            }

            MarketCmd::Cancel(order_id) => {
                book.cancel_order(order_id);
            }

            MarketCmd::Void => {
                info!(market_id = %market_id, "voiding market, cancelling all orders");
                book.cancel_all();
                break;
            }

            MarketCmd::Snapshot(resp) => {
                let _ = resp.send(book.snapshot());
            }

            MarketCmd::Shutdown => {
                info!(market_id = %market_id, "market task shutting down");
                break;
            }
        }
    }

    markets.remove(&market_id);
    metrics::ACTIVE_MARKETS.dec();
    info!(market_id = %market_id, "market task exited");
}

#[cfg(test)]
mod tests {
    use crate::engine::order_book::OrderBook;

    #[test]
    fn order_book_snapshot() {
        let book = OrderBook::new();
        let snap = book.snapshot();
        assert_eq!(snap.best_bid, None);
        assert_eq!(snap.best_ask, None);
        assert_eq!(snap.bid_depth, 0);
        assert_eq!(snap.ask_depth, 0);
        assert_eq!(snap.seq, 0);
    }
}
