use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Duration;

use tokio::signal;
use tokio_util::sync::CancellationToken;
use tonic::transport::Server as TonicServer;
use tracing::{error, info};
use tracing_subscriber::EnvFilter;

mod api;
mod cache;
mod config;
mod domain;
mod engine;
mod events;
mod market;
mod metrics;
mod store;

use api::grpc::server::{pb::matching_service_server::MatchingServiceServer, MatchingGrpcServer};
use api::http::router;
use cache::redis::RedisCache;
use config::Config;
use events::{consumer::KafkaConsumer, publisher::Publisher};
use market::manager::MarketManager;
use store::{db, trade_repo::TradeRepo, wal::Wal};

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    // ── Config ────────────────────────────────────────────────────────────────
    let cfg = Config::from_env().unwrap_or_else(|e| {
        eprintln!("FATAL: config error: {}", e);
        std::process::exit(1);
    });

    // ── Logging ───────────────────────────────────────────────────────────────
    tracing_subscriber::fmt()
        .with_env_filter(
            EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| EnvFilter::new(&cfg.log_level)),
        )
        .json()
        .init();

    info!(version = env!("CARGO_PKG_VERSION"), "matching-engine starting");

    // ── Metrics (force lazy init) ─────────────────────────────────────────────
    metrics::init();

    // ── PostgreSQL ────────────────────────────────────────────────────────────
    let pool = db::new_pool(&cfg.database_url, cfg.database_max_connections)
        .await
        .unwrap_or_else(|e| {
            error!(error = %e, "failed to connect to PostgreSQL");
            std::process::exit(1);
        });

    db::run_migrations(&pool).await.unwrap_or_else(|e| {
        error!(error = %e, "migrations failed");
        std::process::exit(1);
    });

    // ── Redis ─────────────────────────────────────────────────────────────────
    let _redis = RedisCache::new(&cfg.redis_url).await.unwrap_or_else(|e| {
        error!(error = %e, "failed to connect to Redis");
        std::process::exit(1);
    });

    // ── RocksDB WAL ───────────────────────────────────────────────────────────
    let wal = Arc::new(Wal::open(&cfg.wal_path).unwrap_or_else(|e| {
        error!(error = %e, "failed to open RocksDB WAL");
        std::process::exit(1);
    }));

    // ── Kafka publisher ───────────────────────────────────────────────────────
    let publisher = Arc::new(
        Publisher::new(&cfg.kafka_brokers, cfg.kafka_topic_trades_matched.clone())
            .unwrap_or_else(|e| {
                error!(error = %e, "failed to create Kafka publisher");
                std::process::exit(1);
            }),
    );

    // ── Trade repo ────────────────────────────────────────────────────────────
    let trade_repo = Arc::new(TradeRepo::new(pool));

    // ── Market manager ────────────────────────────────────────────────────────
    let market_manager = Arc::new(MarketManager::new(
        publisher,
        trade_repo,
        wal,
        cfg.amm_liquidity_b,
        cfg.amm_default_outcomes,
    ));

    // ── Cancellation token ────────────────────────────────────────────────────
    let cancel = CancellationToken::new();

    // ── Kafka consumer ────────────────────────────────────────────────────────
    let consumer = Arc::new(
        KafkaConsumer::new(
            &cfg.kafka_brokers,
            &cfg.kafka_group_id,
            &cfg.kafka_topic_orders_placed,
            &cfg.kafka_topic_market_voided,
            market_manager.clone(),
        )
        .unwrap_or_else(|e| {
            error!(error = %e, "failed to create Kafka consumer");
            std::process::exit(1);
        }),
    );

    let consumer_cancel = cancel.clone();
    tokio::spawn(async move {
        consumer.run(consumer_cancel).await;
    });

    // ── gRPC server ───────────────────────────────────────────────────────────
    let grpc_addr: SocketAddr = format!("0.0.0.0:{}", cfg.grpc_port).parse()?;
    let grpc_service = MatchingGrpcServer::new(market_manager.clone());

    let grpc_cancel = cancel.clone();
    tokio::spawn(async move {
        info!(addr = %grpc_addr, "gRPC server listening");
        if let Err(e) = TonicServer::builder()
            .add_service(MatchingServiceServer::new(grpc_service))
            .serve_with_shutdown(grpc_addr, grpc_cancel.cancelled())
            .await
        {
            error!(error = %e, "gRPC server error");
        }
    });

    // ── HTTP server ───────────────────────────────────────────────────────────
    let http_addr: SocketAddr = format!("0.0.0.0:{}", cfg.port).parse()?;
    let app = router::build();
    let listener = tokio::net::TcpListener::bind(http_addr).await?;

    let http_cancel = cancel.clone();
    tokio::spawn(async move {
        info!(addr = %http_addr, "HTTP server listening");
        if let Err(e) = axum::serve(listener, app)
            .with_graceful_shutdown(http_cancel.cancelled())
            .await
        {
            error!(error = %e, "HTTP server error");
        }
    });

    // ── Graceful shutdown ─────────────────────────────────────────────────────
    shutdown_signal().await;
    info!("shutdown signal received, draining...");
    cancel.cancel();

    // Allow in-flight operations to complete
    tokio::time::sleep(Duration::from_secs(5)).await;

    market_manager.shutdown().await;
    info!("matching-engine stopped");
    Ok(())
}

async fn shutdown_signal() {
    let ctrl_c = async {
        signal::ctrl_c().await.expect("failed to install Ctrl+C handler");
    };

    #[cfg(unix)]
    let terminate = async {
        signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("failed to install SIGTERM handler")
            .recv()
            .await;
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c => {},
        _ = terminate => {},
    }
}
