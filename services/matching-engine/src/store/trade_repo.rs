use sqlx::PgPool;
use uuid::Uuid;

use crate::domain::trade::{MatchType, Trade};

pub struct TradeRepo {
    pool: PgPool,
}

impl TradeRepo {
    pub fn new(pool: PgPool) -> Self {
        Self { pool }
    }

    /// Persist a trade. Idempotent via ON CONFLICT DO NOTHING.
    pub async fn insert(&self, trade: &Trade) -> anyhow::Result<()> {
        let match_type_str = match trade.match_type {
            MatchType::Clob => "clob",
            MatchType::Amm => "amm",
        };

        sqlx::query!(
            r#"
            INSERT INTO trades
                (id, market_id, buyer_id, seller_id, price_minor, quantity,
                 outcome_index, match_type, seq_no, matched_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8::match_type, $9, $10)
            ON CONFLICT (id) DO NOTHING
            "#,
            trade.id,
            trade.market_id,
            trade.buyer_id,
            trade.seller_id,
            trade.price_minor as i64,
            trade.quantity as i64,
            trade.outcome_index as i32,
            match_type_str,
            trade.seq_no as i64,
            trade.matched_at,
        )
        .execute(&self.pool)
        .await?;

        Ok(())
    }

    /// List trades for a market ordered by sequence number.
    pub async fn list_by_market(&self, market_id: Uuid, limit: i64) -> anyhow::Result<Vec<TradeRow>> {
        let rows = sqlx::query_as!(
            TradeRow,
            r#"
            SELECT id, market_id, buyer_id, seller_id, price_minor, quantity,
                   outcome_index, seq_no, matched_at
            FROM trades
            WHERE market_id = $1
            ORDER BY seq_no ASC
            LIMIT $2
            "#,
            market_id,
            limit,
        )
        .fetch_all(&self.pool)
        .await?;

        Ok(rows)
    }
}

/// A trade row returned from PostgreSQL (flattened, no enum complexity).
pub struct TradeRow {
    pub id: Uuid,
    pub market_id: Uuid,
    pub buyer_id: Uuid,
    pub seller_id: Option<Uuid>,
    pub price_minor: i64,
    pub quantity: i64,
    pub outcome_index: i32,
    pub seq_no: i64,
    pub matched_at: chrono::DateTime<chrono::Utc>,
}
