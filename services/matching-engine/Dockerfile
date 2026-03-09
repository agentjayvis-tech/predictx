# ── Build stage ────────────────────────────────────────────────────────────────
FROM rust:1.79-slim AS builder

# rdkafka requires cmake + clang for cmake-build feature
RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake \
    clang \
    libclang-dev \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Cache dependency compilation separately from source
COPY Cargo.toml Cargo.lock ./
COPY build.rs ./
COPY proto/ proto/

# Create a dummy main so cargo can fetch and compile deps
RUN mkdir -p src && echo "fn main() {}" > src/main.rs
RUN cargo build --release 2>/dev/null || true

# Now build the real binary
COPY src/ src/
COPY migrations/ migrations/
RUN touch src/main.rs && cargo build --release

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libssl3 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Non-root user for security
RUN adduser --disabled-password --uid 1001 --gecos "" appuser

WORKDIR /app
COPY --from=builder /app/target/release/matching-engine /usr/local/bin/matching-engine
COPY --from=builder /app/migrations /migrations

# WAL directory
RUN mkdir -p /data/wal && chown appuser:appuser /data/wal

USER appuser

ENV PORT=8004 \
    GRPC_PORT=9004 \
    LOG_LEVEL=info \
    WAL_PATH=/data/wal \
    DATABASE_MAX_CONNECTIONS=20 \
    AMM_LIQUIDITY_B=100.0 \
    AMM_DEFAULT_OUTCOMES=2

EXPOSE 8004 9004

ENTRYPOINT ["matching-engine"]
