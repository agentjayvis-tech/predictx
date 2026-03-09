# ─── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Download dependencies (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Build the binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o order-service ./cmd/server

# ─── Runtime stage ────────────────────────────────────────────────────────────────
FROM alpine:3.19

# Install minimal runtime dependencies
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1001 appuser

# Copy binary and migrations from builder
COPY --from=builder /build/order-service /usr/local/bin/order-service
COPY --from=builder /build/migrations /migrations

# Run as non-root user
USER appuser

# Set default environment variables
ENV PORT=8003 \
    GRPC_PORT=9003 \
    LOG_LEVEL=info

EXPOSE 8003 9003

ENTRYPOINT ["order-service"]
