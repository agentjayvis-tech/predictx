package service

import (
	"context"
	"time"

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
// Validates deposit limits and exclusion status before allowing deposit.
func (s *WalletService) Deposit(ctx context.Context, userID uuid.UUID, currency domain.Currency, amountMinor int64, idempotencyKey, description string) (*domain.Transaction, error) {
	// Check deposit eligibility (limits, cool-off, self-exclusion)
	if err := s.checkDepositEligibility(ctx, userID, currency, amountMinor); err != nil {
		return nil, err
	}

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
	go s.runFraudCheck(wallet, txn)

	// Publish event (non-blocking in terms of the caller; publisher is synchronous internally for durability).
	go s.runPublishEvent(txn, wallet.BalanceMinor)

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
	go s.runFraudCheck(wallet, txn)

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
	go s.runFraudCheck(wallet, txn)

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
	go s.runFraudCheck(wallet, txn)
	go s.runPublishEvent(txn, wallet.BalanceMinor)

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

// runFraudCheck wraps fraud detection in a goroutine with panic recovery.
func (s *WalletService) runFraudCheck(wallet *domain.Wallet, txn *domain.Transaction) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("fraud check panicked",
				zap.Any("panic", r),
				zap.String("user_id", wallet.UserID.String()),
				zap.String("txn_id", txn.ID.String()),
			)
		}
	}()
	s.fraud.Check(context.Background(), wallet, txn)
}

// runPublishEvent wraps event publishing in a goroutine with panic recovery.
func (s *WalletService) runPublishEvent(txn *domain.Transaction, balanceMinor int64) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("event publish panicked",
				zap.Any("panic", r),
				zap.String("txn_id", txn.ID.String()),
			)
		}
	}()
	s.publisher.PublishPaymentCompleted(context.Background(), txn, balanceMinor)
}

// ─── Responsible Gambling Methods ─────────────────────────────────────────────

// checkDepositEligibility validates that user can deposit given the amount.
// Returns error if user is in cool-off, self-excluded, or exceeds daily limit.
func (s *WalletService) checkDepositEligibility(ctx context.Context, userID uuid.UUID, currency domain.Currency, amountMinor int64) error {
	// Check exclusion settings first (cool-off, self-exclusion)
	es, err := s.repo.GetExclusionSettings(ctx, userID)
	if err != nil {
		s.log.Error("failed to fetch exclusion settings", zap.String("user_id", userID.String()), zap.Error(err))
		return err
	}

	now := time.Now()
	if es.IsInCoolOff(now) {
		return domain.ErrInCoolOffPeriod
	}
	if es.IsSelfExcluded {
		return domain.ErrSelfExcluded
	}

	// Check deposit limits
	ds, err := s.repo.GetDepositSettings(ctx, userID)
	if err != nil {
		s.log.Error("failed to fetch deposit settings", zap.String("user_id", userID.String()), zap.Error(err))
		return err
	}

	if !ds.Enabled {
		return domain.ErrDepositLimitExceeded
	}

	// Check daily limit
	dailyTotal, err := s.repo.GetDailyDepositTotal(ctx, userID, now)
	if err != nil {
		s.log.Error("failed to fetch daily deposit total", zap.String("user_id", userID.String()), zap.Error(err))
		return err
	}

	if (dailyTotal + amountMinor) > ds.DailyDepositLimitMinor {
		return domain.ErrDepositLimitExceeded
	}

	// Check monthly limit if set
	if ds.MonthlyDepositLimitMinor != nil {
		// For simplicity, assume we'd need to sum this month's deposits
		// For now, only check if set but skip actual calculation
		// This would need a separate method to get monthly total
	}

	return nil
}

// CheckOrderEligibility checks if user can place an order (not in cool-off or self-excluded).
// Used by order-service via gRPC.
func (s *WalletService) CheckOrderEligibility(ctx context.Context, userID uuid.UUID) error {
	es, err := s.repo.GetExclusionSettings(ctx, userID)
	if err != nil {
		return err
	}

	now := time.Now()
	if es.IsInCoolOff(now) {
		return domain.ErrInCoolOffPeriod
	}
	if es.IsSelfExcluded {
		return domain.ErrSelfExcluded
	}
	return nil
}

// GetDepositSettings returns user's deposit limit configuration.
func (s *WalletService) GetDepositSettings(ctx context.Context, userID uuid.UUID) (*domain.DepositSettings, error) {
	return s.repo.GetDepositSettings(ctx, userID)
}

// UpdateDepositSettings updates user's deposit limit configuration.
func (s *WalletService) UpdateDepositSettings(ctx context.Context, userID uuid.UUID, dailyLimitMinor int64, monthlyLimitMinor *int64) (*domain.DepositSettings, error) {
	ds := &domain.DepositSettings{
		DailyDepositLimitMinor:   dailyLimitMinor,
		MonthlyDepositLimitMinor: monthlyLimitMinor,
	}
	if err := ds.Validate(); err != nil {
		return nil, err
	}
	return s.repo.UpdateDepositSettings(ctx, userID, dailyLimitMinor, monthlyLimitMinor)
}

// GetExclusionSettings returns user's cool-off and self-exclusion configuration.
func (s *WalletService) GetExclusionSettings(ctx context.Context, userID uuid.UUID) (*domain.ExclusionSettings, error) {
	return s.repo.GetExclusionSettings(ctx, userID)
}

// StartCoolOff initiates a cool-off period for the user.
func (s *WalletService) StartCoolOff(ctx context.Context, userID uuid.UUID, durationHours int) (*domain.ExclusionSettings, error) {
	// Validate duration
	if durationHours != 24 && durationHours != 168 && durationHours != 720 {
		return nil, domain.ErrInvalidCoolOffDuration
	}

	es, err := s.repo.GetExclusionSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	coolOffUntil := now.Add(time.Duration(durationHours) * time.Hour)
	es.CoolOffUntil = &coolOffUntil
	es.CoolOffDurationHours = &durationHours
	// Determine cancellability based on country
	es.CoolOffCancellable = s.isCoolOffCancellableForCountry(ctx, es.CountryCode)

	return s.repo.UpdateExclusionSettings(ctx, userID, es)
}

// CancelCoolOff attempts to cancel an active cool-off period.
// Returns error if cancellation is not allowed in user's region.
func (s *WalletService) CancelCoolOff(ctx context.Context, userID uuid.UUID) (*domain.ExclusionSettings, error) {
	es, err := s.repo.GetExclusionSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	if !es.CoolOffCancellable {
		return nil, domain.ErrCoolOffNotCancellable
	}

	es.CoolOffUntil = nil
	es.CoolOffDurationHours = nil
	es.CoolOffCancellable = false

	return s.repo.UpdateExclusionSettings(ctx, userID, es)
}

// SelfExclude initiates self-exclusion for the user.
// If durationDays is nil, exclusion is permanent.
func (s *WalletService) SelfExclude(ctx context.Context, userID uuid.UUID, durationDays *int) (*domain.ExclusionSettings, error) {
	es, err := s.repo.GetExclusionSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	es.IsSelfExcluded = true
	es.SelfExcludedAt = &now
	es.SelfExclusionDurationDays = durationDays

	return s.repo.UpdateExclusionSettings(ctx, userID, es)
}

// CancelSelfExclusion attempts to cancel self-exclusion (only for temporary exclusions).
// Returns error if exclusion is permanent.
func (s *WalletService) CancelSelfExclusion(ctx context.Context, userID uuid.UUID) (*domain.ExclusionSettings, error) {
	es, err := s.repo.GetExclusionSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	if es.IsPermanentlySelfExcluded() {
		return nil, domain.ErrInvalidSelfExclusion
	}

	es.IsSelfExcluded = false
	es.SelfExcludedAt = nil
	es.SelfExclusionDurationDays = nil

	return s.repo.UpdateExclusionSettings(ctx, userID, es)
}

// isCoolOffCancellableForCountry checks if user's country allows cool-off cancellation.
func (s *WalletService) isCoolOffCancellableForCountry(ctx context.Context, countryCode *string) bool {
	if countryCode == nil {
		return false  // Default: no cancellation
	}

	policy, err := s.repo.GetCountryRGPolicy(ctx, *countryCode)
	if err != nil {
		s.log.Warn("failed to fetch country policy", zap.String("country_code", *countryCode), zap.Error(err))
		return false
	}

	if policy == nil {
		return false  // No policy found; default to no cancellation
	}

	return policy.CoolOffCancellable
}
