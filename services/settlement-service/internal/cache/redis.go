package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// PositionCache stores position data in Redis for fast reads.
// TTL is 120s — positions change infrequently after matching.
type PositionCache struct {
	client *redis.Client
	ttl    time.Duration
}

type positionEntry struct {
	Data     []byte    `json:"data"`
	CachedAt time.Time `json:"cached_at"`
}

func NewPositionCache(client *redis.Client, ttlSecs int) *PositionCache {
	return &PositionCache{
		client: client,
		ttl:    time.Duration(ttlSecs) * time.Second,
	}
}

func positionKey(userID, marketID uuid.UUID, outcomeIndex int) string {
	return fmt.Sprintf("position:%s:%s:%d", userID, marketID, outcomeIndex)
}

func marketPositionsKey(marketID uuid.UUID) string {
	return fmt.Sprintf("positions:market:%s", marketID)
}

// Get returns cached position JSON bytes for a specific (user, market, outcome) triple.
func (c *PositionCache) Get(ctx context.Context, userID, marketID uuid.UUID, outcomeIndex int) ([]byte, bool) {
	raw, err := c.client.Get(ctx, positionKey(userID, marketID, outcomeIndex)).Bytes()
	if err != nil {
		return nil, false
	}
	var entry positionEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, false
	}
	return entry.Data, true
}

// Set writes the serialized position to Redis.
func (c *PositionCache) Set(ctx context.Context, userID, marketID uuid.UUID, outcomeIndex int, data []byte) {
	entry := positionEntry{Data: data, CachedAt: time.Now()}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	c.client.Set(ctx, positionKey(userID, marketID, outcomeIndex), raw, c.ttl) //nolint:errcheck
}

// InvalidateUser removes a cached position for a specific user/market/outcome.
func (c *PositionCache) Invalidate(ctx context.Context, userID, marketID uuid.UUID, outcomeIndex int) {
	c.client.Del(ctx, positionKey(userID, marketID, outcomeIndex)) //nolint:errcheck
}

// InvalidateMarket removes all cached positions for a market (used on settlement).
func (c *PositionCache) InvalidateMarket(ctx context.Context, marketID uuid.UUID) {
	// Use SCAN to find all keys for this market (more reliable than pattern delete).
	pattern := fmt.Sprintf("position:*:%s:*", marketID)
	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil || len(keys) == 0 {
		return
	}
	c.client.Del(ctx, keys...) //nolint:errcheck
	c.client.Del(ctx, marketPositionsKey(marketID)) //nolint:errcheck
}

// NewRedisClient creates a go-redis client from a Redis URL.
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
