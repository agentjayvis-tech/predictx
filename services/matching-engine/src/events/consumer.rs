use std::sync::Arc;

use rdkafka::consumer::{CommitMode, Consumer, StreamConsumer};
use rdkafka::message::Message;
use rdkafka::ClientConfig;
use tracing::{error, info, warn};

use crate::events::types::{MarketVoidedEvent, OrderPlacedEvent};
use crate::market::manager::MarketManager;

pub struct KafkaConsumer {
    orders_placed: StreamConsumer,
    market_voided: StreamConsumer,
    market_manager: Arc<MarketManager>,
}

impl KafkaConsumer {
    pub fn new(
        brokers: &str,
        group_id: &str,
        topic_orders_placed: &str,
        topic_market_voided: &str,
        market_manager: Arc<MarketManager>,
    ) -> anyhow::Result<Self> {
        let make_consumer = |topics: &[&str]| -> anyhow::Result<StreamConsumer> {
            let c: StreamConsumer = ClientConfig::new()
                .set("bootstrap.servers", brokers)
                .set("group.id", group_id)
                .set("enable.auto.commit", "false")
                .set("auto.offset.reset", "earliest")
                .set("session.timeout.ms", "30000")
                .set("max.poll.interval.ms", "300000")
                .create()?;
            c.subscribe(topics)?;
            Ok(c)
        };

        Ok(Self {
            orders_placed: make_consumer(&[topic_orders_placed])?,
            market_voided: make_consumer(&[topic_market_voided])?,
            market_manager,
        })
    }

    /// Run both consumer loops concurrently. Exits when ctx is cancelled.
    pub async fn run(self: Arc<Self>, ctx: tokio::sync::CancellationToken) {
        let orders_task = {
            let consumer = Arc::clone(&self);
            let ctx2 = ctx.clone();
            tokio::spawn(async move { consumer.consume_orders_placed(ctx2).await })
        };

        let voided_task = {
            let consumer = Arc::clone(&self);
            let ctx2 = ctx.clone();
            tokio::spawn(async move { consumer.consume_market_voided(ctx2).await })
        };

        let _ = tokio::join!(orders_task, voided_task);
        info!("Kafka consumers stopped");
    }

    async fn consume_orders_placed(&self, ctx: tokio::sync::CancellationToken) {
        info!("started consuming orders.placed");
        loop {
            tokio::select! {
                _ = ctx.cancelled() => break,
                result = self.orders_placed.recv() => {
                    match result {
                        Err(e) => error!(error = %e, "orders.placed: recv error"),
                        Ok(msg) => {
                            let payload = msg.payload().unwrap_or_default();
                            match serde_json::from_slice::<OrderPlacedEvent>(payload) {
                                Err(e) => warn!(error = %e, "orders.placed: deserialize error"),
                                Ok(event) => {
                                    if let Some(order) = event.into_order() {
                                        self.market_manager.submit_order(order).await;
                                    } else {
                                        warn!("orders.placed: invalid order event, skipping");
                                    }
                                }
                            }
                            if let Err(e) = self.orders_placed.commit_message(&msg, CommitMode::Async) {
                                warn!(error = %e, "orders.placed: commit failed");
                            }
                        }
                    }
                }
            }
        }
    }

    async fn consume_market_voided(&self, ctx: tokio::sync::CancellationToken) {
        info!("started consuming market.voided");
        loop {
            tokio::select! {
                _ = ctx.cancelled() => break,
                result = self.market_voided.recv() => {
                    match result {
                        Err(e) => error!(error = %e, "market.voided: recv error"),
                        Ok(msg) => {
                            let payload = msg.payload().unwrap_or_default();
                            match serde_json::from_slice::<MarketVoidedEvent>(payload) {
                                Err(e) => warn!(error = %e, "market.voided: deserialize error"),
                                Ok(event) => {
                                    self.market_manager.void_market(event.market_id).await;
                                    info!(market_id = %event.market_id, "voided market, all orders cancelled");
                                }
                            }
                            if let Err(e) = self.market_voided.commit_message(&msg, CommitMode::Async) {
                                warn!(error = %e, "market.voided: commit failed");
                            }
                        }
                    }
                }
            }
        }
    }
}
