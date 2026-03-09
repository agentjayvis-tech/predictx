use thiserror::Error;
use uuid::Uuid;

#[derive(Debug, Error)]
pub enum EngineError {
    #[error("market {0} not found")]
    MarketNotFound(Uuid),

    #[error("order {0} not found")]
    OrderNotFound(Uuid),

    #[error("order {0} already cancelled or filled")]
    OrderNotCancellable(Uuid),

    #[error("invalid price {0}: must be 1–99")]
    InvalidPrice(u64),

    #[error("invalid quantity {0}: must be > 0")]
    InvalidQuantity(u64),

    #[error("market channel closed for market {0}")]
    MarketChannelClosed(Uuid),

    #[error("kafka error: {0}")]
    Kafka(String),

    #[error("database error: {0}")]
    Database(#[from] sqlx::Error),

    #[error("wal error: {0}")]
    Wal(String),

    #[error("redis error: {0}")]
    Redis(#[from] redis::RedisError),

    #[error("serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    #[error("internal error: {0}")]
    Internal(String),
}

pub type EngineResult<T> = Result<T, EngineError>;
