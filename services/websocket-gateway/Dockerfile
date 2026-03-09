FROM elixir:1.14-alpine AS builder

RUN apk add --no-cache git build-base

WORKDIR /app

# Copy mix files
COPY mix.exs mix.lock* ./
RUN mix local.hex --force && \
    mix local.rebar --force && \
    MIX_ENV=prod mix deps.get --only prod

# Copy source
COPY . .
RUN MIX_ENV=prod mix compile

# Release stage
FROM elixir:1.14-alpine

RUN apk add --no-cache openssl bash

WORKDIR /app

# Copy compiled app from builder
COPY --from=builder /app/_build/prod/rel/websocket_gateway /app/

EXPOSE 8005 9005

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8005/healthz || exit 1

CMD ["/app/bin/websocket_gateway", "start"]
