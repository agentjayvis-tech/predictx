package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/predictx/wallet-service/internal/cache"
	"github.com/predictx/wallet-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func TestFraudLargeCreditDetection(t *testing.T) {
	ctx := context.Background()
	repo := NewMockWalletRepo()
	balCache := cache.NewBalanceCache(redis.NewClient(&redis.Options{Addr: "localhost:6379"}), 5)
	fraud := NewFraudService(repo, balCache, zap.NewNop(), 10, 100000, 80)

	wallet := &domain.Wallet{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		Currency:     domain.CurrencyCoins,
		BalanceMinor: 0,
		IsActive:     true,
	}

	txn := &domain.Transaction{
		ID:          uuid.New(),
		UserID:      wallet.UserID,
		TxnType:     domain.TxnTypeDeposit,
		Currency:    domain.CurrencyCoins,
		AmountMinor: 150000, // > threshold of 100000
	}

	// Should not panic; alert is logged/persisted
	fraud.Check(ctx, wallet, txn)
}

func TestFraudRapidDrainDetection(t *testing.T) {
	ctx := context.Background()
	repo := NewMockWalletRepo()
	balCache := cache.NewBalanceCache(redis.NewClient(&redis.Options{Addr: "localhost:6379"}), 5)
	fraud := NewFraudService(repo, balCache, zap.NewNop(), 10, 100000, 80)

	wallet := &domain.Wallet{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		Currency:     domain.CurrencyCoins,
		BalanceMinor: 100, // Drops to 10 = 90% drain
		IsActive:     true,
	}

	// Store snapshot of 100
	balCache.SnapshotBalance(ctx, wallet.UserID, wallet.ID, 100)

	txn := &domain.Transaction{
		ID:          uuid.New(),
		UserID:      wallet.UserID,
		TxnType:     domain.TxnTypeSpend,
		Currency:    domain.CurrencyCoins,
		AmountMinor: 90,
	}

	fraud.Check(ctx, wallet, txn)
	// Should detect drain; alert persisted in repo (no-op in mock)
}
