# ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Download dependencies first (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Build the binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o market-service ./cmd/server

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1001 appuser

COPY --from=builder /build/market-service /usr/local/bin/market-service
COPY --from=builder /build/migrations /migrations

USER appuser

ENV PORT=8001 \
    GRPC_PORT=9001 \
    LOG_LEVEL=info

EXPOSE 8001 9001

CMD ["market-service"]
