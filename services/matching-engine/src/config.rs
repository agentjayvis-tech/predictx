use serde::Deserialize;

#[derive(Debug, Deserialize, Clone)]
pub struct Config {
    // HTTP
    #[serde(default = "default_port")]
    pub port: u16,

    // gRPC
    #[serde(default = "default_grpc_port")]
    pub grpc_port: u16,

    // PostgreSQL
    pub database_url: String,
    #[serde(default = "default_db_max_conns")]
    pub database_max_connections: u32,

    // Redis
    pub redis_url: String,

    // Kafka
    pub kafka_brokers: String,
    #[serde(default = "default_group_id")]
    pub kafka_group_id: String,
    #[serde(default = "default_topic_orders_placed")]
    pub kafka_topic_orders_placed: String,
    #[serde(default = "default_topic_trades_matched")]
    pub kafka_topic_trades_matched: String,
    #[serde(default = "default_topic_market_voided")]
    pub kafka_topic_market_voided: String,

    // AMM
    #[serde(default = "default_amm_b")]
    pub amm_liquidity_b: f64,
    #[serde(default = "default_amm_outcomes")]
    pub amm_default_outcomes: usize,

    // RocksDB WAL
    #[serde(default = "default_wal_path")]
    pub wal_path: String,

    // Logging
    #[serde(default = "default_log_level")]
    pub log_level: String,
}

impl Config {
    pub fn from_env() -> anyhow::Result<Self> {
        dotenvy::dotenv().ok(); // load .env if present; ignore if missing
        let cfg = envy::from_env::<Config>()?;
        cfg.validate()?;
        Ok(cfg)
    }

    fn validate(&self) -> anyhow::Result<()> {
        if self.database_url.is_empty() {
            anyhow::bail!("DATABASE_URL is required");
        }
        if self.redis_url.is_empty() {
            anyhow::bail!("REDIS_URL is required");
        }
        if self.kafka_brokers.is_empty() {
            anyhow::bail!("KAFKA_BROKERS is required");
        }
        if self.database_max_connections == 0 || self.database_max_connections > 500 {
            anyhow::bail!("DATABASE_MAX_CONNECTIONS must be 1–500");
        }
        if self.amm_liquidity_b <= 0.0 {
            anyhow::bail!("AMM_LIQUIDITY_B must be positive");
        }
        if self.amm_default_outcomes < 2 {
            anyhow::bail!("AMM_DEFAULT_OUTCOMES must be >= 2");
        }
        Ok(())
    }
}

fn default_port() -> u16 { 8004 }
fn default_grpc_port() -> u16 { 9004 }
fn default_db_max_conns() -> u32 { 20 }
fn default_group_id() -> String { "matching-engine".into() }
fn default_topic_orders_placed() -> String { "orders.placed".into() }
fn default_topic_trades_matched() -> String { "trades.matched".into() }
fn default_topic_market_voided() -> String { "market.voided".into() }
fn default_amm_b() -> f64 { 100.0 }
fn default_amm_outcomes() -> usize { 2 }
fn default_wal_path() -> String { "/data/wal".into() }
fn default_log_level() -> String { "info".into() }

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn defaults_are_sane() {
        assert_eq!(default_port(), 8004);
        assert_eq!(default_grpc_port(), 9004);
        assert_eq!(default_amm_b(), 100.0);
        assert!(default_amm_outcomes() >= 2);
    }
}
