package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// MarketCache stores market data in Redis for fast reads.
// TTL is 60s by default — stale reads acceptable for listing; single-market reads tolerate 60s lag.
type MarketCache struct {
	client *redis.Client
	ttl    time.Duration
}

type marketEntry struct {
	Data     []byte    `json:"data"`
	CachedAt time.Time `json:"cached_at"`
}

func NewMarketCache(client *redis.Client, ttlSecs int) *MarketCache {
	return &MarketCache{
		client: client,
		ttl:    time.Duration(ttlSecs) * time.Second,
	}
}

func marketKey(marketID uuid.UUID) string {
	return fmt.Sprintf("market:%s", marketID)
}

// Get returns the raw JSON bytes of a cached market, or (nil, false) on miss.
func (c *MarketCache) Get(ctx context.Context, marketID uuid.UUID) ([]byte, bool) {
	raw, err := c.client.Get(ctx, marketKey(marketID)).Bytes()
	if err != nil {
		return nil, false
	}
	var entry marketEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, false
	}
	return entry.Data, true
}

// Set writes the serialized market to Redis with the configured TTL.
func (c *MarketCache) Set(ctx context.Context, marketID uuid.UUID, data []byte) {
	entry := marketEntry{Data: data, CachedAt: time.Now()}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	c.client.Set(ctx, marketKey(marketID), raw, c.ttl) //nolint:errcheck
}

// Invalidate removes a cached market entry (called on status transitions).
func (c *MarketCache) Invalidate(ctx context.Context, marketID uuid.UUID) {
	c.client.Del(ctx, marketKey(marketID)) //nolint:errcheck
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
