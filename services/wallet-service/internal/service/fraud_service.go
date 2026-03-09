package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/predictx/wallet-service/internal/cache"
	"github.com/predictx/wallet-service/internal/domain"
	"github.com/predictx/wallet-service/internal/repository"
	"go.uber.org/zap"
)

// FraudService detects suspicious wallet activity using three sliding-window rules.
type FraudService struct {
	repo                *repository.WalletRepo
	balCache            *cache.BalanceCache
	log                 *zap.Logger
	maxChangesPerMin    int
	largeCreditThreshold int64
	rapidDrainPct       int
}

func NewFraudService(
	repo *repository.WalletRepo,
	balCache *cache.BalanceCache,
	log *zap.Logger,
	maxChangesPerMin int,
	largeCreditThreshold int64,
	rapidDrainPct int,
) *FraudService {
	return &FraudService{
		repo:                 repo,
		balCache:             balCache,
		log:                  log,
		maxChangesPerMin:     maxChangesPerMin,
		largeCreditThreshold: largeCreditThreshold,
		rapidDrainPct:        rapidDrainPct,
	}
}

// Check runs all fraud rules against the completed transaction.
// It is called asynchronously in a goroutine; errors are only logged.
func (f *FraudService) Check(ctx context.Context, wallet *domain.Wallet, txn *domain.Transaction) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Rule 1: Rapid balance changes (>maxChangesPerMin in 60s)
	f.checkRapidChanges(ctx, wallet, txn)

	// Rule 2: Large single credit
	if txn.TxnType == domain.TxnTypeDeposit || txn.TxnType == domain.TxnTypePayout {
		f.checkLargeCredit(ctx, wallet, txn)
	}

	// Rule 3: Rapid balance drain (debit operations only)
	if txn.TxnType == domain.TxnTypeSpend {
		f.checkRapidDrain(ctx, wallet, txn)
	}
}

func (f *FraudService) checkRapidChanges(ctx context.Context, wallet *domain.Wallet, txn *domain.Transaction) {
	count, err := f.balCache.IncrChangeCounter(ctx, wallet.UserID, wallet.ID, 60)
	if err != nil {
		f.log.Warn("fraud: incr change counter", zap.Error(err))
		return
	}
	if count > int64(f.maxChangesPerMin) {
		f.saveAlert(ctx, wallet, domain.FraudAlertRapidChanges, map[string]any{
			"count":      count,
			"window_sec": 60,
			"txn_id":     txn.ID.String(),
		})
	}
}

func (f *FraudService) checkLargeCredit(ctx context.Context, wallet *domain.Wallet, txn *domain.Transaction) {
	if txn.AmountMinor >= f.largeCreditThreshold {
		f.saveAlert(ctx, wallet, domain.FraudAlertLargeCredit, map[string]any{
			"amount_minor": txn.AmountMinor,
			"threshold":    f.largeCreditThreshold,
			"txn_id":       txn.ID.String(),
		})
	}
}

func (f *FraudService) checkRapidDrain(ctx context.Context, wallet *domain.Wallet, txn *domain.Transaction) {
	// Store a snapshot of the balance at the moment of first spend (SetNX — only if not already set).
	// The snapshot has a 5-minute TTL; if it exists, compare current balance against it.
	snapshotBal, ok := f.balCache.GetSnapshotBalance(ctx, wallet.UserID, wallet.ID)
	if !ok {
		// No snapshot yet — store current balance before the spend.
		// Use balance before this transaction (balance + amount).
		balBefore := wallet.BalanceMinor + txn.AmountMinor
		f.balCache.SnapshotBalance(ctx, wallet.UserID, wallet.ID, balBefore)
		return
	}

	if snapshotBal == 0 {
		return
	}

	dropPct := int(float64(snapshotBal-wallet.BalanceMinor) / float64(snapshotBal) * 100)
	if dropPct >= f.rapidDrainPct {
		f.saveAlert(ctx, wallet, domain.FraudAlertRapidDrain, map[string]any{
			"snapshot_balance": snapshotBal,
			"current_balance":  wallet.BalanceMinor,
			"drop_pct":         dropPct,
			"txn_id":           txn.ID.String(),
		})
	}
}

func (f *FraudService) saveAlert(ctx context.Context, wallet *domain.Wallet, alertType domain.FraudAlertType, details map[string]any) {
	alert := &domain.FraudAlert{
		ID:        uuid.New(),
		UserID:    wallet.UserID,
		WalletID:  wallet.ID,
		AlertType: alertType,
		Details:   details,
	}
	if err := f.repo.InsertFraudAlert(ctx, alert); err != nil {
		f.log.Error("fraud: insert alert", zap.Error(err))
		return
	}
	f.log.Warn("fraud: alert raised",
		zap.String("type", string(alertType)),
		zap.String("user_id", wallet.UserID.String()),
		zap.String("detail", fmt.Sprintf("%v", details)),
	)
}
