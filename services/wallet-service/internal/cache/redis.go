package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/predictx/wallet-service/internal/domain"
	"github.com/redis/go-redis/v9"
)

// BalanceCache stores wallet balances in Redis for fast reads.
// TTL is intentionally short (5s) — stale reads are acceptable for display;
// gRPC always reads from PostgreSQL for financial operations.
type BalanceCache struct {
	client *redis.Client
	ttl    time.Duration
}

type balanceEntry struct {
	BalanceMinor int64     `json:"balance_minor"`
	CachedAt     time.Time `json:"cached_at"`
}

func NewBalanceCache(client *redis.Client, ttlSecs int) *BalanceCache {
	return &BalanceCache{
		client: client,
		ttl:    time.Duration(ttlSecs) * time.Second,
	}
}

func key(userID uuid.UUID, currency domain.Currency) string {
	return fmt.Sprintf("wallet:balance:%s:%s", userID, string(currency))
}

// Get returns (balance, true) on cache hit, or (0, false) on miss/error.
func (c *BalanceCache) Get(ctx context.Context, userID uuid.UUID, currency domain.Currency) (int64, bool) {
	raw, err := c.client.Get(ctx, key(userID, currency)).Bytes()
	if err != nil {
		return 0, false
	}
	var entry balanceEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return 0, false
	}
	return entry.BalanceMinor, true
}

// Set writes the balance to Redis with the configured TTL.
func (c *BalanceCache) Set(ctx context.Context, userID uuid.UUID, currency domain.Currency, balance int64) {
	entry := balanceEntry{BalanceMinor: balance, CachedAt: time.Now()}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	c.client.Set(ctx, key(userID, currency), raw, c.ttl) //nolint:errcheck
}

// Invalidate removes the cached balance for a wallet (called on every write).
func (c *BalanceCache) Invalidate(ctx context.Context, userID uuid.UUID, currency domain.Currency) {
	c.client.Del(ctx, key(userID, currency)) //nolint:errcheck
}

// IncrChangeCounter increments the sliding-window change counter for fraud detection.
// Returns the new count. The key expires after windowSecs.
func (c *BalanceCache) IncrChangeCounter(ctx context.Context, userID, walletID uuid.UUID, windowSecs int) (int64, error) {
	k := fmt.Sprintf("wallet:fraud:changes:%s:%s", userID, walletID)
	pipe := c.client.Pipeline()
	incr := pipe.Incr(ctx, k)
	pipe.Expire(ctx, k, time.Duration(windowSecs)*time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// SnapshotBalance stores a balance snapshot for rapid-drain detection (5-min window).
func (c *BalanceCache) SnapshotBalance(ctx context.Context, userID, walletID uuid.UUID, balance int64) {
	k := fmt.Sprintf("wallet:fraud:snapshot:%s:%s", userID, walletID)
	// Only set if key doesn't exist (preserve the 5-min-ago snapshot).
	c.client.SetNX(ctx, k, balance, 5*time.Minute) //nolint:errcheck
}

// GetSnapshotBalance retrieves the 5-minute-old balance for drain detection.
// Returns (0, false) if no snapshot exists.
func (c *BalanceCache) GetSnapshotBalance(ctx context.Context, userID, walletID uuid.UUID) (int64, bool) {
	k := fmt.Sprintf("wallet:fraud:snapshot:%s:%s", userID, walletID)
	val, err := c.client.Get(ctx, k).Int64()
	if err != nil {
		return 0, false
	}
	return val, true
}

// NewRedisClient creates a go-redis client from the given URL.
func NewRedisClient(url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	return client, nil
}
