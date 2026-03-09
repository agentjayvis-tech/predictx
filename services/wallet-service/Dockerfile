# Stage 1: Build
FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o wallet-service ./cmd/server

# Stage 2: Runtime
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1001 appuser

COPY --from=builder /build/wallet-service /usr/local/bin/wallet-service
COPY --from=builder /build/migrations /migrations

USER appuser

ENV PORT=8002 \
    GRPC_PORT=9002 \
    LOG_LEVEL=info

EXPOSE 8002 9002

CMD ["wallet-service"]
