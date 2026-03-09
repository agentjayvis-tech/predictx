package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/predictx/order-service/internal/domain"
)

// OrderCache provides caching for orders and rate limiting via Redis.
type OrderCache struct {
	client      *redis.Client
	orderTTL    time.Duration
	rgLimitTTL  time.Duration
	log         *zap.Logger
}

// NewOrderCache creates a new order cache.
func NewOrderCache(client *redis.Client, orderTTLSecs, rgLimitTTLSecs int, log *zap.Logger) *OrderCache {
	return &OrderCache{
		client:     client,
		orderTTL:   time.Duration(orderTTLSecs) * time.Second,
		rgLimitTTL: time.Duration(rgLimitTTLSecs) * time.Second,
		log:        log,
	}
}

// Get retrieves a value from cache.
func (c *OrderCache) Get(ctx context.Context, key string) (interface{}, bool) {
	raw, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false
	}
	if err != nil {
		c.log.Warn("cache get error", zap.String("key", key), zap.Error(err))
		return nil, false
	}

	var entry cacheEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		c.log.Warn("cache unmarshal error", zap.String("key", key), zap.Error(err))
		return nil, false
	}

	return entry.Data, true
}

// Set stores a value in cache with TTL.
func (c *OrderCache) Set(ctx context.Context, key string, value interface{}, ttlSecs int) {
	entry := cacheEntry{
		Data:     value,
		CachedAt: time.Now(),
	}

	raw, err := json.Marshal(entry)
	if err != nil {
		c.log.Warn("cache marshal error", zap.String("key", key), zap.Error(err))
		return
	}

	ttl := time.Duration(ttlSecs) * time.Second
	if err := c.client.Set(ctx, key, raw, ttl).Err(); err != nil {
		c.log.Warn("cache set error", zap.String("key", key), zap.Error(err))
	}
}

// Invalidate removes a key from cache.
func (c *OrderCache) Invalidate(ctx context.Context, key string) {
	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.log.Warn("cache delete error", zap.String("key", key), zap.Error(err))
	}
}

// Increment increments a counter for rate limiting (sliding window).
func (c *OrderCache) Increment(ctx context.Context, key string, expireSecs int) (int64, error) {
	pipe := c.client.Pipeline()

	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Duration(expireSecs)*time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}

	return incr.Val(), nil
}

// cacheEntry wraps cached data with metadata.
type cacheEntry struct {
	Data     interface{} `json:"data"`
	CachedAt time.Time   `json:"cached_at"`
}

// Helper functions for specific cache operations

// GetOrder retrieves a cached order.
func (c *OrderCache) GetOrder(ctx context.Context, orderID string) (*domain.Order, bool) {
	key := fmt.Sprintf("order:%s", orderID)
	if data, ok := c.Get(ctx, key); ok {
		if order, ok := data.(*domain.Order); ok {
			return order, true
		}
	}
	return nil, false
}

// SetOrder stores an order in cache.
func (c *OrderCache) SetOrder(ctx context.Context, orderID string, order *domain.Order) {
	key := fmt.Sprintf("order:%s", orderID)
	c.Set(ctx, key, order, int(c.orderTTL.Seconds()))
}

// InvalidateOrder removes an order from cache.
func (c *OrderCache) InvalidateOrder(ctx context.Context, orderID string) {
	key := fmt.Sprintf("order:%s", orderID)
	c.Invalidate(ctx, key)
}

// GetRateLimit retrieves the rate limit counter for a user.
func (c *OrderCache) GetRateLimit(ctx context.Context, userID string) (int64, error) {
	key := fmt.Sprintf("user:%s:orders", userID)
	val, err := c.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// NewRedisClient creates a new Redis client.
func NewRedisClient(redisURL string) (*redis.Client, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)

	// Test connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return client, nil
}
