use std::time::Duration;

use rdkafka::producer::{FutureProducer, FutureRecord};
use rdkafka::ClientConfig;
use tracing::{error, info};

use crate::domain::trade::Trade;
use crate::events::types::TradeMatchedEvent;

pub struct Publisher {
    producer: FutureProducer,
    topic_trades_matched: String,
}

impl Publisher {
    pub fn new(brokers: &str, topic_trades_matched: String) -> anyhow::Result<Self> {
        let producer: FutureProducer = ClientConfig::new()
            .set("bootstrap.servers", brokers)
            .set("message.timeout.ms", "5000")
            .set("queue.buffering.max.messages", "100000")
            .set("queue.buffering.max.kbytes", "1048576")
            .set("batch.num.messages", "500")
            .set("linger.ms", "1") // low latency: don't wait long to batch
            .create()?;

        info!(brokers = brokers, topic = %topic_trades_matched, "Kafka producer ready");
        Ok(Self { producer, topic_trades_matched })
    }

    /// Publish a matched trade to `trades.matched`.
    /// Key = market_id (ensures same-market trades land on same partition → ordering).
    pub async fn publish_trade(&self, trade: &Trade) -> anyhow::Result<()> {
        let event = TradeMatchedEvent::from(trade);
        let payload = serde_json::to_vec(&event)?;
        let key = trade.market_id.to_string();

        self.producer
            .send(
                FutureRecord::to(&self.topic_trades_matched)
                    .key(key.as_str())
                    .payload(&payload),
                Duration::from_secs(5),
            )
            .await
            .map_err(|(e, _)| anyhow::anyhow!("kafka produce failed: {}", e))?;

        Ok(())
    }
}
