# Order Service

Core transaction engine for PredictX prediction markets. Handles order placement, validation, and lifecycle management.

## Features

✅ **Order Validation**
- Market active status + bounds validation
- User balance verification (gRPC to Wallet Service)
- Outcome index bounds checking

✅ **Responsible Gambling (RG) Enforcement**
- Daily limit: 10,000 coins
- Weekly limit: 50,000 coins
- Auto-reset at midnight/Monday

✅ **Rate Limiting**
- Per-user sliding window: 100 orders/min
- Redis-backed for performance

✅ **Event-Driven Architecture**
- Publish: `orders.placed`, `orders.cancelled`
- Consume: `market.voided` → auto-cancel pending orders
- Kafka integration for async processing

✅ **APIs**
- gRPC (port 9003) for services
- REST HTTP (port 8003) for frontend

## Architecture

```
Order Service
├── Domain Layer (order.go, errors.go)
│   └── Order, RGLimit, sentinels
├── Repository Layer (order_repo.go)
│   └── pgx async database operations
├── Service Layer
│   ├── order_service.go (business logic)
│   ├── rate_limiter.go (100/min per user)
│   └── rg_service.go (daily/weekly limits)
├── API Layer
│   ├── gRPC server (order_server.go)
│   └── HTTP handlers (order_handler.go)
├── Infrastructure
│   ├── Kafka (publisher.go, consumer.go)
│   ├── Redis (order_cache.go)
│   └── PostgreSQL (migrations)
└── Config (config.go)
```

## Quick Start

### Prerequisites
- PostgreSQL 17
- Redis 8
- Kafka (for events)
- Go 1.22+

### Local Development

```bash
# Setup environment
cp .env.example .env

# Run dependencies
docker-compose up -d postgres redis kafka

# Install dependencies
go mod download

# Run migrations
go run ./cmd/server -migrate-only

# Run tests
go test ./...

# Start service
go run ./cmd/server/main.go
```

### Docker

```bash
docker build -t order-service:latest .
docker run -p 8003:8003 -p 9003:9003 \
  --env-file .env \
  order-service:latest
```

## API Examples

### Create Order (gRPC)

```bash
grpcurl -plaintext \
  -d '{
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "market_id": "550e8400-e29b-41d4-a716-446655440001",
    "order_type": "buy",
    "price_minor": 100,
    "outcome_index": 0,
    "idempotency_key": "order-123"
  }' \
  localhost:9003 orderservice.OrderService/CreateOrder
```

### Create Order (HTTP)

```bash
curl -X POST http://localhost:8003/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "market_id": "550e8400-e29b-41d4-a716-446655440001",
    "order_type": "buy",
    "price_minor": 100,
    "outcome_index": 0,
    "idempotency_key": "order-123"
  }'
```

### Get Order

```bash
curl http://localhost:8003/v1/orders/550e8400-e29b-41d4-a716-446655440000
```

### Cancel Order

```bash
curl -X POST http://localhost:8003/v1/orders/550e8400-e29b-41d4-a716-446655440000/cancel \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "idempotency_key": "cancel-123"
  }'
```

## Configuration

See `.env.example` for all config options:

```bash
# Server
PORT=8003
GRPC_PORT=9003
LOG_LEVEL=info

# Database
DATABASE_URL=postgres://...
DATABASE_MAX_CONNS=20

# Redis
REDIS_URL=redis://...
REDIS_ORDER_TTL_SECONDS=60

# Kafka
KAFKA_BROKERS=localhost:9092

# gRPC Services
WALLET_SERVICE_ADDR=wallet-service:9002
MARKET_SERVICE_ADDR=market-service:9001

# Limits
RATE_LIMIT_MAX_ORDERS_PER_MIN=100
RG_DAILY_LIMIT_COINS=10000
RG_WEEKLY_LIMIT_COINS=50000
```

## Database Schema

### orders table
- id, user_id, market_id
- order_type, status, time_in_force
- price_minor, quantity_shares, currency, outcome_index
- idempotency_key (unique)
- created_at, updated_at

### rg_limits table
- id, user_id (unique)
- daily_spent_minor, weekly_spent_minor
- daily_reset_at, weekly_reset_at
- updated_at

## Kafka Topics

**Publish:**
- `orders.placed` - order creation event
- `orders.cancelled` - order cancellation event

**Consume:**
- `markets.voided` - auto-cancel pending orders on voided markets
- `orders.matched` - (future) settlement integration

## Validation Flow

1. ✅ Rate limit check (Redis INCR)
2. ✅ Market validation (Market Service gRPC)
3. ✅ Outcome index bounds check
4. ✅ Balance check (Wallet Service gRPC)
5. ✅ RG limits check (daily/weekly)
6. ✅ Persist order (idempotent)
7. ✅ Publish event (non-blocking)

## Testing

```bash
# Unit tests (no external dependencies)
go test ./internal/... -v

# Integration tests (requires Docker services)
go test ./... -count=1 -race

# Coverage
go test ./... -cover
```

## Integration Points

- **Wallet Service** (gRPC): CheckBalance() before order creation
- **Market Service** (gRPC): GetMarket() for validation
- **Matching Engine** (Kafka): Consumes orders.placed events
- **Resolution Service** (Kafka): Publishes markets.voided events

## Performance

- Order creation: <100ms (with cache hits)
- Rate limiting: Redis O(1) per request
- Database: pgx async with connection pooling
- Caching: 60s order TTL, 30-min RG limit TTL

## Monitoring

- Structured logging (Zap) with request IDs
- Prometheus metrics (future)
- Health check: GET /health

## License

See repo LICENSE file.
