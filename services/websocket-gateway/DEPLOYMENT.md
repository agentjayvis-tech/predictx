# WebSocket Gateway Deployment Guide

## Local Development

### Prerequisites

- Elixir 1.14+
- OTP 26+
- Docker & Docker Compose (optional)

### Quick Start

```bash
# Clone and navigate
cd services/websocket-gateway

# Install dependencies
mix deps.get

# Start with Docker Compose
docker-compose up

# OR start manually
export REDIS_HOST=localhost KAFKA_BROKERS=localhost:9092 JWT_SECRET=dev_secret
iex -S mix phx.server
```

Server runs at `http://localhost:8005`

## Testing Locally

```bash
# Unit tests
mix test

# With coverage
mix test --cover

# Code quality
mix credo --strict

# Check formatting
mix format --check-formatted
```

## Docker Build

```bash
# Build image
docker build -t predictx/websocket-gateway:latest .

# Run container
docker run -p 8005:8005 \
  -e JWT_SECRET=your_secret \
  -e REDIS_HOST=redis-host \
  -e KAFKA_BROKERS=kafka-broker:9092 \
  predictx/websocket-gateway:latest
```

## Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: websocket-gateway
  labels:
    app: websocket-gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app: websocket-gateway
  template:
    metadata:
      labels:
        app: websocket-gateway
    spec:
      containers:
      - name: websocket-gateway
        image: predictx/websocket-gateway:latest
        ports:
        - containerPort: 8005
          name: http
        - containerPort: 9005
          name: metrics
        env:
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: websocket-gateway-secrets
              key: jwt-secret
        - name: REDIS_HOST
          value: "redis"
        - name: KAFKA_BROKERS
          value: "kafka-0.kafka-headless:9092,kafka-1.kafka-headless:9092"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8005
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8005
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            cpu: 250m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 512Mi
---
apiVersion: v1
kind: Service
metadata:
  name: websocket-gateway
spec:
  selector:
    app: websocket-gateway
  type: LoadBalancer
  ports:
  - protocol: TCP
    port: 8005
    targetPort: 8005
    name: http
  - protocol: TCP
    port: 9005
    targetPort: 9005
    name: metrics
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HOST` | `0.0.0.0` | Bind address |
| `PORT` | `8005` | HTTP port |
| `JWT_SECRET` | required | JWT signing secret |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `KAFKA_BROKERS` | `localhost:9092` | Kafka broker addresses |
| `MIX_ENV` | `dev` | Environment (dev/test/prod) |

## Health Checks

```bash
# HTTP health check
curl http://localhost:8005/healthz

# Redis connectivity
redis-cli -h localhost PING

# Kafka consumer status
# Check logs for consumer group status
```

## Monitoring

### Metrics

Telemetry metrics are published to stdout in production. Set up a metrics collector:

```bash
# View metrics
mix run -e "IO.puts(:metrics)"
```

### Logs

```bash
# View recent logs
docker logs websocket-gateway

# Stream logs
docker logs -f websocket-gateway
```

## Scaling

### Horizontal Scaling

Deploy multiple replicas behind a load balancer:

```
Load Balancer
    ↓↓↓
[Gateway 1] [Gateway 2] [Gateway 3]
    ↓↓↓
[Shared Redis] [Shared Kafka]
```

### Connection Limits

Each instance can handle ~500 concurrent connections. For 10,000 concurrent users:
- Deploy 20 instances
- Ensure Redis can handle pub/sub for all markets
- Monitor Kafka consumer lag

## Troubleshooting

### High Latency

```bash
# Check Redis latency
redis-cli --latency

# Check Kafka consumer lag
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group websocket-gateway --describe

# Check application metrics
tail -f logs/websocket-gateway.log | grep latency_ms
```

### Connection Drops

- Increase `KAFKA_FETCH_MAX_WAIT_TIME` if consumer can't keep up
- Check network connectivity between services
- Monitor memory usage (OOM kills connections)

### Memory Leaks

```bash
# Monitor Erlang VM
iex> :observer.start()

# Check process count
iex> length(Process.list())

# Check chat message cache
redis-cli INFO memory
```

## Upgrading

### Rolling Update

```bash
# 1. Build new image
docker build -t predictx/websocket-gateway:v0.2.0 .

# 2. Push to registry
docker push predictx/websocket-gateway:v0.2.0

# 3. Update Kubernetes deployment
kubectl set image deployment/websocket-gateway \
  websocket-gateway=predictx/websocket-gateway:v0.2.0

# 4. Monitor rollout
kubectl rollout status deployment/websocket-gateway
```

## Backup & Recovery

- **Chat history**: Stored in Redis with 100-message limit per market. No backup needed.
- **State**: All state is ephemeral. No persistent data to backup.

## Security

- JWT secrets must be at least 32 characters
- Use HTTPS in production (CloudFlare, ALB, etc.)
- Restrict Kafka topic access
- Monitor for suspicious JWT claims
