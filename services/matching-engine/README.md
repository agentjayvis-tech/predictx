# Matching Engine

High-performance prediction market order matching engine for PredictX.

## Architecture

**CLOB + AMM Hybrid:**
- **Central Limit Order Book (CLOB)** for high-volume markets: price-time priority, two-sided matching, sub-microsecond match loop
- **LMSR AMM fallback** for low-volume markets: guaranteed liquidity at any price, odds recalculate on each fill
- **Single-threaded per market** via Tokio `mpsc` channels — eliminates lock contention, ensures deterministic sequence numbers

**Performance:**
- Match latency target: **< 10ms p99** (typically < 1ms in-memory)
- Throughput: **10,000+ orders/sec per market**
- Load tested to **1,000 concurrent users**
- Prometheus histogram: `matching_engine_match_latency_ms`

**Event Sourcing:**
- Every trade is published to Kafka `trades.matched` (partitioned by `market_id`)
- Local RocksDB WAL for crash recovery without Kafka replay
- Sequence numbers ensure deterministic ordering

## Prerequisites

- Rust 1.79+ (`rustup update stable`)
- PostgreSQL 17
- Redis 8
- Kafka
- CMake + Clang (required by `rdkafka` cmake-build feature)

## Quick Start

```bash
# Install system dependencies (Ubuntu/Debian)
sudo apt-get install cmake clang libclang-dev libssl-dev

# Copy and edit config
cp .env.example .env

# Start dependencies
docker compose up -d postgres redis kafka

# Run database migrations
cargo run -- --migrate-only   # (or migrations apply on startup automatically)

# Run tests
cargo test --lib

# Start service
cargo run

# Release build
cargo build --release
./target/release/matching-engine
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8004` | HTTP port (healthz, metrics) |
| `GRPC_PORT` | `9004` | gRPC port |
| `DATABASE_URL` | — | PostgreSQL 17 connection string |
| `DATABASE_MAX_CONNECTIONS` | `20` | pgx pool size |
| `REDIS_URL` | — | Redis 8 connection string |
| `KAFKA_BROKERS` | — | Comma-separated broker list |
| `KAFKA_GROUP_ID` | `matching-engine` | Consumer group |
| `KAFKA_TOPIC_ORDERS_PLACED` | `orders.placed` | Inbound orders |
| `KAFKA_TOPIC_MARKET_VOIDED` | `market.voided` | Market void events |
| `KAFKA_TOPIC_TRADES_MATCHED` | `trades.matched` | Outbound trades |
| `AMM_LIQUIDITY_B` | `100.0` | LMSR liquidity parameter |
| `AMM_DEFAULT_OUTCOMES` | `2` | Default outcomes per market |
| `WAL_PATH` | `/data/wal` | RocksDB WAL directory |
| `LOG_LEVEL` | `info` | trace/debug/info/warn/error |

## API

### HTTP

| Endpoint | Description |
|----------|-------------|
| `GET /healthz` | Health check — returns `200 ok` |
| `GET /metrics` | Prometheus metrics scrape |

### gRPC (port 9004)

```bash
# Install grpcurl
brew install grpcurl

# Get order book for a market
grpcurl -plaintext \
  -d '{"market_id": "550e8400-e29b-41d4-a716-446655440001"}' \
  localhost:9004 matching.MatchingService/GetOrderBook

# Get current odds
grpcurl -plaintext \
  -d '{"market_id": "550e8400-e29b-41d4-a716-446655440001"}' \
  localhost:9004 matching.MatchingService/GetMarketOdds

# Stream live trades
grpcurl -plaintext \
  -d '{"market_id": "550e8400-e29b-41d4-a716-446655440001"}' \
  localhost:9004 matching.MatchingService/StreamTrades
```

## Kafka Topics

| Topic | Direction | Description |
|-------|-----------|-------------|
| `orders.placed` | Consumed | Orders from Order Service |
| `market.voided` | Consumed | Market void events (cancels all resting orders) |
| `trades.matched` | Published | Matched trades → Settlement Service, WebSocket Gateway |

## Matching Logic

### CLOB (price-time priority)

For binary prediction markets, YES@P matches NO@(100-P):
- YES orders (bids) stored in descending price order
- NO orders (asks) stored in descending price order
- Match when `best_bid + best_ask >= 100`
- Fill at bid price (maker wins spread)
- Partial fills: larger order stays in book

### AMM (LMSR) Fallback

When no CLOB counterparty exists:
```
cost(q) = b × ln(Σ exp(qᵢ/b))
p_i     = exp(qᵢ/b) / Σ exp(qⱼ/b)
```
- `b = AMM_LIQUIDITY_B` (platform liquidity parameter)
- Limit orders: only fill if AMM price ≤ limit price
- Market orders: always fill against AMM
- Odds update after each fill

## Testing

```bash
# Unit tests (no external dependencies)
cargo test --lib

# Engine tests specifically
cargo test engine::

# AMM tests
cargo test amm::

# With output
cargo test -- --nocapture
```

## Integration Points

- **Order Service** (Go, port 8003): publishes `orders.placed`
- **Settlement Service** (Go, port 8005): consumes `trades.matched` to settle payouts
- **WebSocket Gateway** (Elixir): consumes `trades.matched` for real-time odds updates
- **Prometheus**: scrapes `GET /metrics` for SLO dashboards

## Docker

```bash
docker build -t matching-engine:latest .
docker run \
  -p 8004:8004 -p 9004:9004 \
  -v /data/wal:/data/wal \
  --env-file .env \
  matching-engine:latest
```
