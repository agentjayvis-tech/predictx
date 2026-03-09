use once_cell::sync::Lazy;
use prometheus::{
    register_counter_vec, register_gauge, register_histogram, Counter, CounterVec, Gauge,
    Histogram, HistogramOpts,
};

/// Match latency in milliseconds (buckets optimised for <10ms target).
pub static MATCH_LATENCY_MS: Lazy<Histogram> = Lazy::new(|| {
    register_histogram!(HistogramOpts::new(
        "matching_engine_match_latency_ms",
        "Order match latency in milliseconds"
    )
    .buckets(vec![0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 25.0, 50.0]))
    .unwrap()
});

/// Orders matched, labelled by match_type ("clob" or "amm").
pub static ORDERS_PROCESSED: Lazy<CounterVec> = Lazy::new(|| {
    register_counter_vec!(
        "matching_engine_orders_processed_total",
        "Total orders processed by match type",
        &["match_type"]
    )
    .unwrap()
});

/// Number of active market tasks.
pub static ACTIVE_MARKETS: Lazy<Gauge> = Lazy::new(|| {
    register_gauge!(
        "matching_engine_active_markets",
        "Number of markets with active order book tasks"
    )
    .unwrap()
});

/// Trades successfully published to Kafka.
pub static TRADES_PUBLISHED: Lazy<Counter> = Lazy::new(|| {
    prometheus::register_counter!(
        "matching_engine_trades_published_total",
        "Total trades published to trades.matched Kafka topic"
    )
    .unwrap()
});

/// Force lazy initialization on startup so metrics appear even before first trade.
pub fn init() {
    Lazy::force(&MATCH_LATENCY_MS);
    Lazy::force(&ORDERS_PROCESSED);
    Lazy::force(&ACTIVE_MARKETS);
    Lazy::force(&TRADES_PUBLISHED);
}
