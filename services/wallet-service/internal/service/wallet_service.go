package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/predictx/wallet-service/internal/cache"
	"github.com/predictx/wallet-service/internal/domain"
	"github.com/predictx/wallet-service/internal/events"
	"github.com/predictx/wallet-service/internal/repository"
	"go.uber.org/zap"
)

// WalletService is the primary business logic layer for wallet operations.
type WalletService struct {
	repo      *repository.WalletRepo
	balCache  *cache.BalanceCache
	publisher *events.Publisher
	fraud     *FraudService
	log       *zap.Logger
}

func NewWalletService(
	repo *repository.WalletRepo,
	balCache *cache.BalanceCache,
	publisher *events.Publisher,
	fraud *FraudService,
	log *zap.Logger,
) *WalletService {
	return &WalletService{
		repo:      repo,
		balCache:  balCache,
		publisher: publisher,
		fraud:     fraud,
		log:       log,
	}
}

// GetOrCreateWallet ensures a wallet exists for the given user+currency.
func (s *WalletService) GetOrCreateWallet(ctx context.Context, userID uuid.UUID, currency domain.Currency) (*domain.Wallet, error) {
	if !domain.SupportedCurrencies[currency] {
		return nil, domain.ErrUnsupportedCurrency
	}
	return s.repo.GetOrCreateWallet(ctx, userID, currency)
}

// GetBalance returns the balance for a specific currency, using Redis cache when possible.
func (s *WalletService) GetBalance(ctx context.Context, userID uuid.UUID, currency domain.Currency) (int64, error) {
	if bal, ok := s.balCache.Get(ctx, userID, currency); ok {
		return bal, nil
	}
	wallet, err := s.repo.GetWallet(ctx, userID, currency)
	if err != nil {
		return 0, err
	}
	s.balCache.Set(ctx, userID, currency, wallet.BalanceMinor)
	return wallet.BalanceMinor, nil
}

// GetAllWallets returns all wallets for a user (all currencies).
func (s *WalletService) GetAllWallets(ctx context.Context, userID uuid.UUID) ([]*domain.Wallet, error) {
	return s.repo.GetAllWallets(ctx, userID)
}

// Deposit credits COINS to a user's wallet (admin grant, daily reward, purchase).
// Publishes a payments.completed Kafka event on success.
func (s *WalletService) Deposit(ctx context.Context, userID uuid.UUID, currency domain.Currency, amountMinor int64, idempotencyKey, description string) (*domain.Transaction, error) {
	wallet, err := s.GetOrCreateWallet(ctx, userID, currency)
	if err != nil {
		return nil, err
	}

	txn, err := s.repo.ApplyTransaction(ctx, domain.ApplyTxnRequest{
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		TxnType:        domain.TxnTypeDeposit,
		Currency:       currency,
		AmountMinor:    amountMinor,
		EntryType:      domain.EntryTypeCredit,
		Description:    description,
	})
	if err != nil {
		return nil, err
	}

	s.balCache.Invalidate(ctx, userID, currency)

	// Refresh wallet balance for fraud check.
	wallet.BalanceMinor += amountMinor
	go s.fraud.Check(context.Background(), wallet, txn)

	// Publish event (non-blocking in terms of the caller; publisher is synchronous internally for durability).
	go s.publisher.PublishPaymentCompleted(context.Background(), txn, wallet.BalanceMinor)

	return txn, nil
}

// Spend debits COINS from a user's wallet for bet placement.
func (s *WalletService) Spend(ctx context.Context, userID uuid.UUID, currency domain.Currency, amountMinor int64, idempotencyKey, description string, referenceID *uuid.UUID, referenceType string) (*domain.Transaction, error) {
	wallet, err := s.repo.GetWallet(ctx, userID, currency)
	if err != nil {
		return nil, err
	}

	txn, err := s.repo.ApplyTransaction(ctx, domain.ApplyTxnRequest{
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		TxnType:        domain.TxnTypeSpend,
		Currency:       currency,
		AmountMinor:    amountMinor,
		EntryType:      domain.EntryTypeDebit,
		Description:    description,
		ReferenceID:    referenceID,
		ReferenceType:  referenceType,
	})
	if err != nil {
		return nil, err
	}

	s.balCache.Invalidate(ctx, userID, currency)

	wallet.BalanceMinor -= amountMinor
	go s.fraud.Check(context.Background(), wallet, txn)

	return txn, nil
}

// Refund credits COINS back when a market is voided or disputed.
func (s *WalletService) Refund(ctx context.Context, userID uuid.UUID, currency domain.Currency, amountMinor int64, idempotencyKey, description string, referenceID *uuid.UUID) (*domain.Transaction, error) {
	wallet, err := s.repo.GetOrCreateWallet(ctx, userID, currency)
	if err != nil {
		return nil, err
	}

	txn, err := s.repo.ApplyTransaction(ctx, domain.ApplyTxnRequest{
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		TxnType:        domain.TxnTypeRefund,
		Currency:       currency,
		AmountMinor:    amountMinor,
		EntryType:      domain.EntryTypeCredit,
		Description:    description,
		ReferenceID:    referenceID,
		ReferenceType:  "market",
	})
	if err != nil {
		return nil, err
	}

	s.balCache.Invalidate(ctx, userID, currency)
	wallet.BalanceMinor += amountMinor
	go s.fraud.Check(context.Background(), wallet, txn)

	return txn, nil
}

// Payout credits COINS for a winning prediction.
// Publishes a payments.completed event (consumed by Notification and Compliance).
func (s *WalletService) Payout(ctx context.Context, userID uuid.UUID, currency domain.Currency, amountMinor int64, idempotencyKey, description string, referenceID *uuid.UUID) (*domain.Transaction, error) {
	wallet, err := s.repo.GetOrCreateWallet(ctx, userID, currency)
	if err != nil {
		return nil, err
	}

	txn, err := s.repo.ApplyTransaction(ctx, domain.ApplyTxnRequest{
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		TxnType:        domain.TxnTypePayout,
		Currency:       currency,
		AmountMinor:    amountMinor,
		EntryType:      domain.EntryTypeCredit,
		Description:    description,
		ReferenceID:    referenceID,
		ReferenceType:  "market",
	})
	if err != nil {
		return nil, err
	}

	s.balCache.Invalidate(ctx, userID, currency)
	wallet.BalanceMinor += amountMinor
	go s.fraud.Check(context.Background(), wallet, txn)
	go s.publisher.PublishPaymentCompleted(context.Background(), txn, wallet.BalanceMinor)

	return txn, nil
}

// ListTransactions returns paginated transaction history.
func (s *WalletService) ListTransactions(ctx context.Context, userID uuid.UUID, currency domain.Currency, limit, offset int) ([]*domain.Transaction, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListTransactions(ctx, userID, currency, limit, offset)
}

// CheckBalance returns whether userID has at least amountMinor of currency.
// Always reads from DB for financial accuracy (used by gRPC/Order Service).
func (s *WalletService) CheckBalance(ctx context.Context, userID uuid.UUID, currency domain.Currency, amountMinor int64) (sufficient bool, currentBalance int64, err error) {
	wallet, err := s.repo.GetWallet(ctx, userID, currency)
	if err != nil {
		return false, 0, err
	}
	return wallet.BalanceMinor >= amountMinor, wallet.BalanceMinor, nil
}

// DailyReward grants the configured daily bonus to the user.
func (s *WalletService) DailyReward(ctx context.Context, userID uuid.UUID, coinsAmount int64, idempotencyKey string) (*domain.Transaction, error) {
	wallet, err := s.GetOrCreateWallet(ctx, userID, domain.CurrencyCoins)
	if err != nil {
		return nil, err
	}

	txn, err := s.repo.ApplyTransaction(ctx, domain.ApplyTxnRequest{
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		TxnType:        domain.TxnTypeDailyReward,
		Currency:       domain.CurrencyCoins,
		AmountMinor:    coinsAmount,
		EntryType:      domain.EntryTypeCredit,
		Description:    "Daily login reward",
	})
	if err != nil {
		return nil, err
	}

	s.balCache.Invalidate(ctx, userID, domain.CurrencyCoins)
	s.log.Info("daily_reward granted",
		zap.String("user_id", userID.String()),
		zap.Int64("amount", coinsAmount),
	)
	_ = wallet
	return txn, nil
}
