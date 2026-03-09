# WebSocket Gateway Development Guide

## Architecture Overview

```
Kafka (trades.matched) → KafkaConsumer → Redis Pub/Sub → RedisPublisher →
  Phoenix PubSub → MarketChannel → WebSocket Clients
```

## Project Structure

```
lib/
  websocket_gateway/
    application.ex         # OTP app supervisor
    auth.ex               # JWT verification
    kafka_consumer.ex     # Kafka consumer for trades.matched
    redis_publisher.ex    # Redis Pub/Sub subscriber
  websocket_gateway_web/
    endpoint.ex           # Phoenix endpoint config
    channels/
      user_socket.ex      # WebSocket connection handler
      market_channel.ex    # Market odds + chat
      presence_channel.ex  # User presence tracking
      presence.ex          # Presence module

config/
  config.exs              # Main config

test/
  websocket_gateway_test.exs
  websocket_gateway/
    auth_test.exs
```

## Key Components

### 1. Kafka Consumer (`kafka_consumer.ex`)

Consumes `trades.matched` events from Kafka and publishes to Redis.

```elixir
# Event format from Matching Engine
%{
  "market_id" => "uuid",
  "match_type" => "clob" | "amm",
  "price_minor" => 45,
  "quantity" => 10,
  "buyer_id" => "user1",
  "seller_id" => "user2",
  "timestamp" => 1709981400000
}
```

### 2. Redis Publisher (`redis_publisher.ex`)

Subscribes to `market:*:odds` channels in Redis and broadcasts to Phoenix PubSub.

- **Latency target**: <50ms from Kafka to WebSocket push
- **Pattern**: Uses Redis Pub/Sub for zero-copy fan-out to all instances

### 3. Market Channel (`market_channel.ex`)

Handles WebSocket subscriptions for real-time odds and chat.

**Incoming messages:**
- `chat` — User sends message, broadcast to all subscribers

**Outgoing messages:**
- `odds:updated` — Real-time odds from Kafka
- `chat:new_message` — Chat message from user
- `presence:diff` — User join/leave events

### 4. Presence Tracking (`presence.ex`)

Phoenix Presence module for tracking who's online in each market.

## Development Workflow

### 1. Setup

```bash
# Clone repo
git clone https://github.com/predictx/bet-predict.git
cd services/websocket-gateway

# Install dependencies
mix deps.get

# Create local environment
cp .env.example .env
# Edit .env with local values
```

### 2. Running Locally

```bash
# With Docker Compose (easiest)
docker-compose up

# OR manually start dependencies
# Start Redis: redis-server
# Start Kafka: (see docker-compose.yml)
# Then:
iex -S mix phx.server
```

### 3. Testing WebSocket Connection

```javascript
// JavaScript client
const socket = new WebSocket('ws://localhost:8005/socket?token=jwt_token');
const channel = socket.channel('market:market_123', {});

channel.on('odds:updated', (odds) => {
  console.log('Odds:', odds);
});

channel.push('chat', {message: 'Hello!'});
channel.on('chat:new_message', (msg) => {
  console.log(`${msg.user_name}: ${msg.message}`);
});

channel.join()
  .receive('ok', () => console.log('Joined'))
  .receive('error', (err) => console.log('Error:', err));
```

### 4. Running Tests

```bash
# Unit tests
mix test

# Watch mode
mix test.watch

# With coverage
mix test --cover

# Code quality
mix credo --strict
mix format --check-formatted
```

## Adding Features

### Adding a New Channel

```elixir
# 1. Create channel file
defmodule WebsocketGatewayWeb.NewChannel do
  use Phoenix.Channel

  def join("new:" <> id, _params, socket) do
    {:ok, socket |> assign(:resource_id, id)}
  end

  def handle_in("action", params, socket) do
    # Handle incoming message
    {:noreply, socket}
  end

  def handle_out("broadcast", payload, socket) do
    push(socket, "broadcast", payload)
    {:noreply, socket}
  end
end

# 2. Register in UserSocket
channel "new:*", WebsocketGatewayWeb.NewChannel

# 3. Write tests
defmodule WebsocketGatewayWeb.NewChannelTest do
  use ExUnit.Case
  # Tests here
end
```

### Adding Metrics

```elixir
# Emit metric
:telemetry.execute(
  [:websocket_gateway, :custom, :metric],
  %{value: 42}
)

# Define in application.ex
{Telemetry.Metrics.Counter,
 name: "websocket_gateway.custom.metric"}
```

### Adding Redis Operations

```elixir
# In any module
case Redix.command(:redis, ["SET", "key", "value"]) do
  {:ok, _} -> :ok
  {:error, reason} -> {:error, reason}
end
```

## Performance Optimization

### Reducing Latency

1. **Kafka Consumer**:
   - Increase `fetch_max_wait_time` (default 500ms) if processing slow
   - Reduce if market activity is high

2. **Redis Pub/Sub**:
   - Use pipelining for multiple commands
   - Monitor with `redis-cli --latency`

3. **Channel Broadcasting**:
   - Use `broadcast_from` to exclude sender
   - Batch updates if processing many trades

### Memory Usage

- Each WebSocket connection: ~500KB
- Market chat history: ~100 messages × 1KB = 100KB per market
- For 500 concurrent users + 50 markets: ~300MB Redis

## Debugging

### Enable Debug Logging

```elixir
# In config/config.exs
config :logger, level: :debug

# Or in code
require Logger
Logger.debug("Message: #{inspect(term)}")
```

### Monitor Kafka Consumer

```bash
# Check consumer group
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group websocket-gateway --describe

# Reset offset (if needed)
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group websocket-gateway --topic trades.matched --reset-offsets --to-latest
```

### Monitor Redis

```bash
redis-cli MONITOR          # Watch all commands
redis-cli INFO STATS       # Stats
redis-cli INFO MEMORY      # Memory usage
redis-cli PUBSUB CHANNELS  # Active Pub/Sub channels
```

### Inspect Erlang VM

```elixir
iex> :observer.start()  # GUI monitor
iex> :sys.get_status(pid)  # Process status
iex> Process.info(pid)  # Process info
```

## Common Issues

### "No match of right hand side value :error"

Usually a Kafka/Redis connection failure. Check:
```bash
# Redis
redis-cli PING  # Should return PONG

# Kafka
kafka-topics --bootstrap-server localhost:9092 --list  # Should show topics
```

### High CPU Usage

- Check if Kafka consumer is falling behind (lag increasing)
- Monitor database connections if using persistent state

### Dropped Connections

- Increase `max_frame_size` in endpoint if sending large messages
- Check network MTU size
- Monitor connection pool exhaustion

## Contributing

1. Branch: `git checkout -b feature/my-feature`
2. Code: Follow Credo style
3. Test: `mix test` must pass
4. Format: `mix format`
5. PR: Include description and link to issue

## References

- [Phoenix Channels](https://hexdocs.pm/phoenix/Phoenix.Channel.html)
- [Telemetry](https://hexdocs.pm/telemetry/)
- [Redix](https://hexdocs.pm/redix/)
- [Brod (Kafka)](https://hexdocs.pm/brod/)
