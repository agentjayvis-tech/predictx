use redis::{aio::ConnectionManager, AsyncCommands, Client};
use tracing::info;

const ODDS_TTL_SECS: u64 = 60;

pub struct RedisCache {
    conn: ConnectionManager,
}

impl RedisCache {
    pub async fn new(url: &str) -> anyhow::Result<Self> {
        let client = Client::open(url)?;
        let conn = ConnectionManager::new(client).await?;
        info!(url, "Redis connection established");
        Ok(Self { conn })
    }

    /// Cache current market odds (JSON array of f64). TTL = 60s.
    pub async fn set_market_odds(
        &self,
        market_id: uuid::Uuid,
        odds: &[f64],
    ) -> anyhow::Result<()> {
        let key = format!("me:odds:{}", market_id);
        let value = serde_json::to_string(odds)?;
        let mut conn = self.conn.clone();
        conn.set_ex::<_, _, ()>(key, value, ODDS_TTL_SECS).await?;
        Ok(())
    }

    /// Retrieve cached market odds. Returns None if expired or not set.
    pub async fn get_market_odds(
        &self,
        market_id: uuid::Uuid,
    ) -> anyhow::Result<Option<Vec<f64>>> {
        let key = format!("me:odds:{}", market_id);
        let mut conn = self.conn.clone();
        let raw: Option<String> = conn.get(key).await?;
        match raw {
            Some(s) => Ok(Some(serde_json::from_str(&s)?)),
            None => Ok(None),
        }
    }

    /// Invalidate cached odds for a market after a fill.
    pub async fn invalidate_odds(&self, market_id: uuid::Uuid) -> anyhow::Result<()> {
        let key = format!("me:odds:{}", market_id);
        let mut conn = self.conn.clone();
        conn.del::<_, ()>(key).await?;
        Ok(())
    }

    /// Rate-limit check via sliding window (INCR + EXPIRE).
    /// Returns current count in the current window.
    pub async fn incr_rate_limit(
        &self,
        user_id: uuid::Uuid,
        window_secs: u64,
    ) -> anyhow::Result<u64> {
        let key = format!("me:rl:{}", user_id);
        let mut conn = self.conn.clone();
        let count: u64 = redis::pipe()
            .incr(&key, 1u64)
            .expire(&key, window_secs as i64)
            .ignore()
            .query_async(&mut conn)
            .await?;
        Ok(count)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn odds_cache_key_format() {
        let market_id = uuid::Uuid::new_v4();
        let key = format!("me:odds:{}", market_id);
        assert!(key.starts_with("me:odds:"));
        assert!(key.contains('-')); // UUID has dashes
    }

    #[test]
    fn rate_limit_key_format() {
        let user_id = uuid::Uuid::new_v4();
        let key = format!("me:rl:{}", user_id);
        assert!(key.starts_with("me:rl:"));
        assert!(key.contains('-')); // UUID has dashes
    }

    #[test]
    fn odds_vector_serialization() {
        let odds = vec![0.6, 0.4];
        let json = serde_json::to_string(&odds).expect("should serialize");
        assert_eq!(json, "[0.6,0.4]");
        let deserialized: Vec<f64> = serde_json::from_str(&json).expect("should deserialize");
        assert_eq!(deserialized, odds);
    }

    #[test]
    fn odds_vector_roundtrip_precision() {
        let odds = vec![0.5234567, 0.4765433];
        let json = serde_json::to_string(&odds).expect("should serialize");
        let deserialized: Vec<f64> = serde_json::from_str(&json).expect("should deserialize");
        // Floating point comparison with tolerance
        assert!((deserialized[0] - odds[0]).abs() < 1e-10);
        assert!((deserialized[1] - odds[1]).abs() < 1e-10);
    }
}
