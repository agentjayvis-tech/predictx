use std::path::Path;
use std::sync::Arc;

use rocksdb::{Options, DB};
use tracing::{info, warn};

use crate::domain::trade::Trade;

/// Write-Ahead Log backed by RocksDB.
/// Key format: `{market_id}:{seq_no:020}` → value: JSON-encoded Trade
/// Allows efficient range scans for replay per market.
pub struct Wal {
    db: Arc<DB>,
}

impl Wal {
    pub fn open(path: &str) -> anyhow::Result<Self> {
        let mut opts = Options::default();
        opts.create_if_missing(true);
        opts.set_write_buffer_size(64 * 1024 * 1024); // 64 MB
        opts.set_max_write_buffer_number(3);
        let db = DB::open(&opts, Path::new(path))?;
        info!(path, "RocksDB WAL opened");
        Ok(Self { db: Arc::new(db) })
    }

    /// Append a trade to the WAL. Fast local write — does not block match loop.
    pub fn append(&self, trade: &Trade) -> anyhow::Result<()> {
        let key = wal_key(trade.market_id, trade.seq_no);
        let value = serde_json::to_vec(trade)?;
        self.db.put(key, value)?;
        Ok(())
    }

    /// Replay all trades for a market starting from `from_seq`.
    /// Used during recovery to restore in-memory order book state.
    pub fn replay_from(
        &self,
        market_id: uuid::Uuid,
        from_seq: u64,
    ) -> anyhow::Result<Vec<Trade>> {
        let start = wal_key(market_id, from_seq);
        let end = wal_key(market_id, u64::MAX);

        let iter = self.db.iterator(rocksdb::IteratorMode::From(
            &start,
            rocksdb::Direction::Forward,
        ));

        let mut trades = Vec::new();
        for item in iter {
            let (key, value) = item?;
            // Stop when we've passed the market's key range
            if key.as_ref() >= end.as_slice() {
                break;
            }
            match serde_json::from_slice::<Trade>(&value) {
                Ok(t) => trades.push(t),
                Err(e) => warn!(error = %e, "WAL: skipping corrupt entry"),
            }
        }

        Ok(trades)
    }
}

fn wal_key(market_id: uuid::Uuid, seq_no: u64) -> Vec<u8> {
    // Zero-padded seq_no ensures lexicographic ordering == numeric ordering
    format!("{}:{:020}", market_id, seq_no).into_bytes()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn wal_key_format_zero_padded() {
        let market_id = uuid::Uuid::new_v4();
        let key = wal_key(market_id, 1);
        let key_str = String::from_utf8(key).expect("should be valid utf8");
        // Format should be: {uuid}:{20-digit-padded-seq}
        assert!(key_str.contains(':'));
        let parts: Vec<&str> = key_str.split(':').collect();
        assert_eq!(parts.len(), 2);
        assert_eq!(parts[1].len(), 20); // zero-padded to 20 digits
    }

    #[test]
    fn wal_key_ordering() {
        let market_id = uuid::Uuid::new_v4();
        let key1 = wal_key(market_id, 1);
        let key10 = wal_key(market_id, 10);
        let key100 = wal_key(market_id, 100);

        // Lexicographic order should match numeric order when zero-padded
        assert!(key1 < key10);
        assert!(key10 < key100);
    }

    #[test]
    fn wal_key_different_markets() {
        let market_id_a = uuid::Uuid::new_v4();
        let market_id_b = uuid::Uuid::new_v4();
        let key_a = wal_key(market_id_a, 1);
        let key_b = wal_key(market_id_b, 1);

        // Different markets should have different keys
        assert_ne!(key_a, key_b);
    }
}
