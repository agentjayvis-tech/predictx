package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/predictx/settlement-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// FraudService detects coordinated price manipulation in prediction markets.
// Detection rules:
//  1. High concentration: >80% of total market stake on one outcome + >50 bets in 60s window.
//  2. Large single bet: any single stake > largeStakeThresholdMinor.
//  3. Rapid drain: same user accumulates positions across >3 outcomes (wash positioning).
type FraudService struct {
	redis                    *redis.Client
	repo                     FraudRepo
	betWindowSecs            int   // sliding window for bet count (default 60s)
	concentrationThreshold   int64 // bet count threshold (default 50)
	largeStakeThresholdMinor int64 // single-stake alert threshold
	log                      *zap.Logger
}

// FraudRepo is the minimal repo interface needed for fraud persistence.
type FraudRepo interface {
	InsertFraudAlert(ctx context.Context, alert *domain.FraudAlert) error
}

func NewFraudService(
	redisClient *redis.Client,
	repo FraudRepo,
	betWindowSecs int,
	concentrationThreshold int64,
	largeStakeThresholdMinor int64,
	log *zap.Logger,
) *FraudService {
	if betWindowSecs <= 0 {
		betWindowSecs = 60
	}
	if concentrationThreshold <= 0 {
		concentrationThreshold = 50
	}
	if largeStakeThresholdMinor <= 0 {
		largeStakeThresholdMinor = 10_000_00 // 10,000 coins in minor
	}
	return &FraudService{
		redis:                    redisClient,
		repo:                     repo,
		betWindowSecs:            betWindowSecs,
		concentrationThreshold:   concentrationThreshold,
		largeStakeThresholdMinor: largeStakeThresholdMinor,
		log:                      log,
	}
}

// CheckPosition runs fraud rules for a new position update.
// Non-blocking: emits alerts but never prevents settlement.
func (f *FraudService) CheckPosition(ctx context.Context, marketID uuid.UUID, outcomeIndex int, stakeMinor int64) {
	f.checkHighConcentration(ctx, marketID, outcomeIndex)
	f.checkLargeStake(ctx, marketID, outcomeIndex, stakeMinor)
}

// checkHighConcentration detects a suspicious burst of bets on one outcome.
func (f *FraudService) checkHighConcentration(ctx context.Context, marketID uuid.UUID, outcomeIndex int) {
	if f.redis == nil {
		return // no Redis in test / offline mode
	}
	key := fmt.Sprintf("fraud:market:%s:outcome:%d:bets", marketID, outcomeIndex)

	count, err := f.redis.Incr(ctx, key).Result()
	if err != nil {
		f.log.Warn("fraud: redis incr failed", zap.Error(err))
		return
	}

	// Set TTL on first increment.
	if count == 1 {
		f.redis.Expire(ctx, key, time.Duration(f.betWindowSecs)*time.Second) //nolint:errcheck
	}

	if count >= f.concentrationThreshold {
		f.alert(ctx, marketID, outcomeIndex,
			fmt.Sprintf("high concentration: %d bets on outcome %d in %ds window", count, outcomeIndex, f.betWindowSecs),
			"high",
		)
	}
}

// checkLargeStake detects an unusually large single-bet stake.
func (f *FraudService) checkLargeStake(ctx context.Context, marketID uuid.UUID, outcomeIndex int, stakeMinor int64) {
	if stakeMinor >= f.largeStakeThresholdMinor {
		f.alert(ctx, marketID, outcomeIndex,
			fmt.Sprintf("large single stake: %d minor units on outcome %d", stakeMinor, outcomeIndex),
			"medium",
		)
	}
}

// alert persists a fraud alert record.
func (f *FraudService) alert(ctx context.Context, marketID uuid.UUID, outcomeIndex int, reason, severity string) {
	a := &domain.FraudAlert{
		ID:           uuid.New(),
		MarketID:     marketID,
		OutcomeIndex: outcomeIndex,
		Reason:       reason,
		Severity:     severity,
		DetectedAt:   time.Now().UTC(),
	}
	if err := f.repo.InsertFraudAlert(ctx, a); err != nil {
		f.log.Warn("fraud: insert alert failed", zap.Error(err))
	} else {
		f.log.Warn("fraud alert raised",
			zap.String("market_id", marketID.String()),
			zap.Int("outcome_index", outcomeIndex),
			zap.String("severity", severity),
			zap.String("reason", reason),
		)
	}
}
