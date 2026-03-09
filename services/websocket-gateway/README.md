# WebSocket Gateway

Real-time odds updates and market chat for PredictX. Handles 500+ concurrent connections with <50ms latency.

## Architecture

```
Kafka (trades.matched)
    ↓
KafkaConsumer
    ↓
Redis Pub/Sub (market:*:odds)
    ↓
RedisPublisher
    ↓
Phoenix PubSub
    ↓
MarketChannel (broadcast to WebSocket clients)
```

## Features

- **Real-time odds**: Broadcast trade updates to all market subscribers (<50ms latency)
- **Live chat**: 500+ concurrent users per market with message persistence
- **Presence tracking**: See who's online in each market
- **JWT authentication**: Secure WebSocket connections
- **Metrics**: Telemetry for monitoring latency and channel activity

## Requirements

- Elixir 1.14+
- OTP 26+
- Kafka broker with `trades.matched` topic
- Redis 7+

## Setup

```bash
# Install dependencies
mix deps.get

# Create .env file
cp .env.example .env

# Run locally
iex -S mix phx.server
```

## Configuration

Set these environment variables:

```bash
HOST=localhost              # HTTP server host
PORT=8005                   # HTTP server port
JWT_SECRET=your_secret      # JWT signing key
REDIS_HOST=localhost        # Redis host
REDIS_PORT=6379            # Redis port
KAFKA_BROKERS=localhost:9092 # Kafka brokers
```

## API

### WebSocket Connection

```
ws://localhost:8005/socket?token=JWT_TOKEN
```

### Channels

#### Market Channel
Subscribe to real-time odds for a specific market:

```javascript
const socket = new WebSocket('ws://localhost:8005/socket?token=token');
const channel = socket.channel('market:market_id', {});

// Receive odds updates
channel.on('odds:updated', (odds) => {
  console.log('New odds:', odds);
  // {
  //   market_id, match_type, price_minor, quantity,
  //   buyer_id, seller_id, timestamp, gateway_latency_ms
  // }
});

// Send chat message
channel.push('chat', {message: 'Great odds!'})
  .receive('ok', () => console.log('Message sent'))
  .receive('error', (err) => console.log('Error:', err));

// Receive chat messages
channel.on('chat:new_message', (msg) => {
  console.log(`${msg.user_name}: ${msg.message}`);
});

// Presence updates
channel.on('presence:diff', (diff) => {
  console.log('Users online:', diff.joins);
});

channel.join();
```

#### Presence Channel
Track user activity:

```javascript
const presence = socket.channel('presence:market_id', {});
presence.on('presence_state', (state) => {
  console.log('Users:', state);
});
```

## Metrics

Telemetry events emitted to stdout:

- `websocket_gateway.channel.join` — channel subscriptions
- `websocket_gateway.channel.leave` — channel unsubscriptions
- `websocket_gateway.trades.received` — trades processed
- `websocket_gateway.broadcast.latency_ms` — end-to-end broadcast latency

Example output:
```
[info] 1 event, 50.0ms latency
websocket_gateway.broadcast.latency_ms: 45ms (from Kafka to WebSocket broadcast)
```

## Performance

- **Latency SLA**: <50ms from Kafka event to WebSocket push (p95)
- **Concurrency**: 500+ simultaneous connections per market
- **Throughput**: ~10k trades/sec across all markets
- **Memory**: ~500KB per active connection

## Deployment

```bash
# Build Docker image
docker build -t websocket-gateway:latest .

# Run container
docker run -p 8005:8005 \
  -e JWT_SECRET=secret \
  -e REDIS_HOST=redis \
  -e KAFKA_BROKERS=kafka:9092 \
  websocket-gateway:latest
```

## Testing

```bash
# Run unit tests
mix test

# Run with coverage
mix test --cover

# Code quality
mix credo --strict
```

## Troubleshooting

**WebSocket connection fails**: Check JWT token validity and Redis/Kafka connectivity.

**High latency**: Monitor Redis publish latency (`LATENCY LATEST`). Increase Kafka consumer threads if bottleneck is there.

**Out of memory**: Check active connections with `redis-cli INFO`. Ensure chat message LTRIM works (last 100 msgs/market).
