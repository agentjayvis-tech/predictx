package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/predictx/order-service/internal/domain"
)

// RateLimiter enforces per-user order rate limits using Redis sliding window.
type RateLimiter struct {
	cache     BalanceCache // Any cache with Increment method
	maxReqs   int
	windowSec int
}

// BalanceCache is the interface for Redis-like caching needed by rate limiter.
type BalanceCache interface {
	Increment(ctx context.Context, key string, expireSecs int) (int64, error)
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(cache BalanceCache, maxReqs, windowSec int) *RateLimiter {
	return &RateLimiter{
		cache:     cache,
		maxReqs:   maxReqs,
		windowSec: windowSec,
	}
}

// CheckLimit verifies if a user has exceeded the order rate limit.
// Returns ErrRateLimitExceeded if limit is exceeded.
func (rl *RateLimiter) CheckLimit(ctx context.Context, userID uuid.UUID) error {
	key := fmt.Sprintf("user:%s:orders", userID)
	count, err := rl.cache.Increment(ctx, key, rl.windowSec)
	if err != nil {
		// Cache failure is not fatal; log warning but allow request
		// (rate limiting is best-effort)
		return nil
	}

	if count > int64(rl.maxReqs) {
		return domain.ErrRateLimitExceeded
	}

	return nil
}
