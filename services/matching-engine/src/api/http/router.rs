use axum::{routing::get, Router};
use prometheus::{Encoder, TextEncoder};

pub fn build() -> Router {
    Router::new()
        .route("/healthz", get(health_handler))
        .route("/metrics", get(metrics_handler))
}

async fn health_handler() -> &'static str {
    "ok"
}

async fn metrics_handler() -> Result<String, (axum::http::StatusCode, String)> {
    let encoder = TextEncoder::new();
    let metric_families = prometheus::gather();
    let mut buf = Vec::new();
    encoder
        .encode(&metric_families, &mut buf)
        .map_err(|e| (axum::http::StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    String::from_utf8(buf)
        .map_err(|e| (axum::http::StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}
