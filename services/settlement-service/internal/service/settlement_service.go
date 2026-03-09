package service

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/predictx/settlement-service/internal/domain"
	"go.uber.org/zap"
)

// SettlementRepo is the database interface used by SettlementService.
type SettlementRepo interface {
	UpsertPosition(ctx context.Context, p *domain.Position) (*domain.Position, error)
	GetPosition(ctx context.Context, userID, marketID uuid.UUID, outcomeIndex int) (*domain.Position, error)
	ListPositionsByMarket(ctx context.Context, marketID uuid.UUID) ([]*domain.Position, error)
	ListUserPositions(ctx context.Context, userID uuid.UUID) ([]*domain.Position, error)
	UpdatePositionStatusByMarket(ctx context.Context, marketID uuid.UUID, newStatus, expectedStatus domain.PositionStatus) error
	GetSettlementByMarket(ctx context.Context, marketID uuid.UUID) (*domain.Settlement, error)
	CreateSettlement(ctx context.Context, s *domain.Settlement) (*domain.Settlement, error)
	UpdateSettlementStatus(ctx context.Context, settlementID uuid.UUID, newStatus domain.SettlementStatus) error
	CreateSettlementEntries(ctx context.Context, entries []*domain.SettlementEntry) error
	UpdateEntryStatus(ctx context.Context, entryID uuid.UUID, status domain.EntryStatus) error
	ListPendingEntries(ctx context.Context, settlementID uuid.UUID) ([]*domain.SettlementEntry, error)
	InsertFraudAlert(ctx context.Context, alert *domain.FraudAlert) error
}

// WalletClient is the gRPC interface to Wallet Service.
type WalletClient interface {
	Credit(ctx context.Context, userID, currency string, amountMinor int64, idempotencyKey, description string, referenceID string) error
	Debit(ctx context.Context, userID, currency string, amountMinor int64, idempotencyKey, description string, referenceID string) error
}

// SettlementPublisher publishes settlement events to Kafka.
type SettlementPublisher interface {
	PublishSettlementCompleted(ctx context.Context, settlementID, marketID uuid.UUID, winnerCount int, netPoolMinor int64)
	PublishSettlementVoided(ctx context.Context, marketID uuid.UUID, reason string)
	Close() error
}

// SettlementService implements settlement business logic.
type SettlementService struct {
	repo          SettlementRepo
	wallet        WalletClient
	publisher     SettlementPublisher
	fraud         *FraudService
	cfg           settlementConfig
	log           *zap.Logger
}

type settlementConfig struct {
	platformWalletID string // insurance fund wallet
	currency         string // default "COINS"
	payoutWorkers    int    // max concurrent wallet calls
}

func NewSettlementService(
	repo SettlementRepo,
	wallet WalletClient,
	publisher SettlementPublisher,
	fraud *FraudService,
	platformWalletID, currency string,
	payoutWorkers int,
	log *zap.Logger,
) *SettlementService {
	if payoutWorkers <= 0 {
		payoutWorkers = 10
	}
	return &SettlementService{
		repo:      repo,
		wallet:    wallet,
		publisher: publisher,
		fraud:     fraud,
		cfg: settlementConfig{
			platformWalletID: platformWalletID,
			currency:         currency,
			payoutWorkers:    payoutWorkers,
		},
		log: log,
	}
}

// RecordPosition upserts a user's position from a matched order event.
// Called by the order.matched Kafka consumer.
func (s *SettlementService) RecordPosition(ctx context.Context, userID, marketID uuid.UUID, outcomeIndex int, stakeMinor int64, currency string) (*domain.Position, error) {
	pos := &domain.Position{
		ID:           uuid.New(),
		UserID:       userID,
		MarketID:     marketID,
		OutcomeIndex: outcomeIndex,
		StakeMinor:   stakeMinor,
		Currency:     currency,
		Status:       domain.PositionStatusOpen,
	}

	result, err := s.repo.UpsertPosition(ctx, pos)
	if err != nil {
		return nil, fmt.Errorf("settlement_svc: record_position: %w", err)
	}

	// Async fraud check — does not block position recording.
	go s.runFraudCheck(context.Background(), marketID, outcomeIndex, stakeMinor)

	return result, nil
}

// RecordPositionFromEvent is the adapter used by the Kafka consumer.
// It discards the Position return value since consumers only care about errors.
func (s *SettlementService) RecordPositionFromEvent(ctx context.Context, userID, marketID uuid.UUID, outcomeIndex int, stakeMinor int64, currency string) error {
	_, err := s.RecordPosition(ctx, userID, marketID, outcomeIndex, stakeMinor, currency)
	return err
}

// SettleMarket executes the full settlement saga for a resolved market.
// This is idempotent: if a settlement already exists for this market,
// it resumes from where it left off (pending entries only).
// Triggered by the markets.resolved Kafka event.
func (s *SettlementService) SettleMarket(ctx context.Context, marketID uuid.UUID, winningOutcome int, resolutionID string) error {
	log := s.log.With(zap.String("market_id", marketID.String()), zap.Int("winning_outcome", winningOutcome))

	// ── Phase 1: Check idempotency ────────────────────────────────────────
	existing, err := s.repo.GetSettlementByMarket(ctx, marketID)
	if err != nil && !errors.Is(err, domain.ErrSettlementNotFound) {
		return fmt.Errorf("settlement_svc: check existing: %w", err)
	}
	if existing != nil && existing.Status == domain.SettlementStatusCompleted {
		log.Info("settlement already completed (idempotent)")
		return nil
	}

	var settlement *domain.Settlement

	if existing != nil {
		// Resume existing settlement.
		log.Info("resuming existing settlement", zap.String("settlement_id", existing.ID.String()))
		settlement = existing
	} else {
		// ── Phase 1: Load positions and build settlement ──────────────────
		positions, err := s.repo.ListPositionsByMarket(ctx, marketID)
		if err != nil {
			return fmt.Errorf("settlement_svc: list_positions: %w", err)
		}
		if len(positions) == 0 {
			log.Warn("no positions found for market; nothing to settle")
			return nil
		}

		// Compute pool totals.
		var totalPool, winningStake int64
		var winnerCount, loserCount int
		currency := positions[0].Currency

		for _, p := range positions {
			totalPool += p.StakeMinor
			if p.OutcomeIndex == winningOutcome {
				winningStake += p.StakeMinor
				winnerCount++
			} else {
				loserCount++
			}
		}

		fee := domain.ComputeInsuranceFee(totalPool)
		netPool := totalPool - fee

		// ── Phase 2: Create settlement record (processing) ────────────────
		settlement, err = s.repo.CreateSettlement(ctx, &domain.Settlement{
			ID:                uuid.New(),
			MarketID:          marketID,
			ResolutionID:      resolutionID,
			Status:            domain.SettlementStatusProcessing,
			WinningOutcome:    winningOutcome,
			TotalPoolMinor:    totalPool,
			InsuranceFeeMinor: fee,
			NetPoolMinor:      netPool,
			WinningStakeMinor: winningStake,
			WinnerCount:       winnerCount,
			LoserCount:        loserCount,
			Currency:          currency,
		})
		if err != nil {
			return fmt.Errorf("settlement_svc: create_settlement: %w", err)
		}

		// ── Phase 2: Lock positions to prevent double-settlement ──────────
		if err := s.repo.UpdatePositionStatusByMarket(ctx, marketID,
			domain.PositionStatusSettling, domain.PositionStatusOpen); err != nil {
			return fmt.Errorf("settlement_svc: lock_positions: %w", err)
		}

		// ── Phase 2: Build and persist all settlement entries ─────────────
		entries := make([]*domain.SettlementEntry, 0, len(positions))
		for _, p := range positions {
			isWinner := p.OutcomeIndex == winningOutcome
			var payout int64
			if isWinner {
				payout = domain.ComputePayout(p.StakeMinor, winningStake, netPool)
			}
			entries = append(entries, &domain.SettlementEntry{
				ID:             uuid.New(),
				SettlementID:   settlement.ID,
				UserID:         p.UserID,
				PositionID:     p.ID,
				StakeMinor:     p.StakeMinor,
				PayoutMinor:    payout,
				PnlMinor:       payout - p.StakeMinor,
				IsWinner:       isWinner,
				Status:         domain.EntryStatusPending,
				IdempotencyKey: fmt.Sprintf("sett:%s:user:%s", settlement.ID, p.UserID),
			})
		}

		if err := s.repo.CreateSettlementEntries(ctx, entries); err != nil {
			return fmt.Errorf("settlement_svc: create_entries: %w", err)
		}

		// Collect insurance fee from losers' pool (debit from a virtual market account).
		// In this model the fee is already withheld from NetPool; emit to platform wallet.
		if fee > 0 {
			go s.collectInsuranceFee(context.Background(), settlement, fee)
		}

		log.Info("settlement created",
			zap.String("settlement_id", settlement.ID.String()),
			zap.Int64("total_pool", totalPool),
			zap.Int64("net_pool", netPool),
			zap.Int64("fee", fee),
			zap.Int("winners", winnerCount),
			zap.Int("losers", loserCount),
		)
	}

	// ── Phase 3: Fan out payouts via Wallet gRPC ──────────────────────────
	if err := s.distributePayout(ctx, settlement); err != nil {
		_ = s.repo.UpdateSettlementStatus(ctx, settlement.ID, domain.SettlementStatusFailed)
		return fmt.Errorf("settlement_svc: distribute_payout: %w", err)
	}

	// ── Phase 4: Mark settled ─────────────────────────────────────────────
	if err := s.repo.UpdateSettlementStatus(ctx, settlement.ID, domain.SettlementStatusCompleted); err != nil {
		return fmt.Errorf("settlement_svc: complete_settlement: %w", err)
	}

	if err := s.repo.UpdatePositionStatusByMarket(ctx, marketID,
		domain.PositionStatusSettled, domain.PositionStatusSettling); err != nil {
		log.Warn("failed to mark positions as settled", zap.Error(err))
	}

	go s.publisher.PublishSettlementCompleted(
		context.Background(),
		settlement.ID, marketID,
		settlement.WinnerCount,
		settlement.NetPoolMinor,
	)

	log.Info("settlement completed", zap.String("settlement_id", settlement.ID.String()))
	return nil
}

// RefundMarket refunds all open positions for a voided market.
// Idempotent: positions already refunded are skipped via status check.
// Triggered by the market.voided Kafka event.
func (s *SettlementService) RefundMarket(ctx context.Context, marketID uuid.UUID, reason string) error {
	log := s.log.With(zap.String("market_id", marketID.String()), zap.String("reason", reason))

	positions, err := s.repo.ListPositionsByMarket(ctx, marketID)
	if err != nil {
		return fmt.Errorf("settlement_svc: refund: list_positions: %w", err)
	}
	if len(positions) == 0 {
		log.Info("no positions to refund")
		return nil
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		failed  int
		sem     = make(chan struct{}, s.cfg.payoutWorkers)
	)

	for _, p := range positions {
		if p.Status != domain.PositionStatusOpen {
			continue // already settled or refunded
		}

		wg.Add(1)
		pos := p
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			idempotencyKey := fmt.Sprintf("refund:%s:user:%s", marketID, pos.UserID)
			desc := fmt.Sprintf("Refund for voided market %s (%s)", marketID, reason)

			if err := s.wallet.Credit(
				context.Background(),
				pos.UserID.String(), pos.Currency,
				pos.StakeMinor,
				idempotencyKey, desc,
				marketID.String(),
			); err != nil {
				log.Warn("refund failed",
					zap.String("user_id", pos.UserID.String()),
					zap.Int64("stake", pos.StakeMinor),
					zap.Error(err),
				)
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			log.Info("refunded position",
				zap.String("user_id", pos.UserID.String()),
				zap.Int64("amount", pos.StakeMinor),
			)
		}()
	}

	wg.Wait()

	// Mark positions refunded (best-effort; failed ones stay open for retry).
	_ = s.repo.UpdatePositionStatusByMarket(ctx, marketID,
		domain.PositionStatusRefunded, domain.PositionStatusOpen)

	go s.publisher.PublishSettlementVoided(context.Background(), marketID, reason)

	if failed > 0 {
		return fmt.Errorf("settlement_svc: refund: %d wallet calls failed", failed)
	}
	log.Info("market refunded successfully", zap.Int("position_count", len(positions)))
	return nil
}

// GetSettlement returns the settlement record for a market.
func (s *SettlementService) GetSettlement(ctx context.Context, marketID uuid.UUID) (*domain.Settlement, error) {
	return s.repo.GetSettlementByMarket(ctx, marketID)
}

// GetUserPositions returns all positions for a user.
func (s *SettlementService) GetUserPositions(ctx context.Context, userID uuid.UUID) ([]*domain.Position, error) {
	return s.repo.ListUserPositions(ctx, userID)
}

// GetUserPnL returns the total P&L for a user across all settled positions in a market.
func (s *SettlementService) GetUserPnL(ctx context.Context, marketID, userID uuid.UUID) (int64, error) {
	settlement, err := s.repo.GetSettlementByMarket(ctx, marketID)
	if err != nil {
		return 0, err
	}
	if settlement.Status != domain.SettlementStatusCompleted {
		return 0, fmt.Errorf("settlement not yet complete")
	}

	// Look up all entries for this user in this settlement.
	// We re-derive from positions for simplicity.
	positions, err := s.repo.ListPositionsByMarket(ctx, marketID)
	if err != nil {
		return 0, err
	}

	var pnl int64
	for _, p := range positions {
		if p.UserID != userID {
			continue
		}
		if p.OutcomeIndex == settlement.WinningOutcome {
			payout := domain.ComputePayout(p.StakeMinor, settlement.WinningStakeMinor, settlement.NetPoolMinor)
			pnl += payout - p.StakeMinor
		} else {
			pnl -= p.StakeMinor // loser loses full stake
		}
	}
	return pnl, nil
}

// ─── internal helpers ─────────────────────────────────────────────────────────

// distributePayout fans out winner payouts via Wallet gRPC with bounded concurrency.
func (s *SettlementService) distributePayout(ctx context.Context, settlement *domain.Settlement) error {
	entries, err := s.repo.ListPendingEntries(ctx, settlement.ID)
	if err != nil {
		return fmt.Errorf("list pending entries: %w", err)
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed int
		sem    = make(chan struct{}, s.cfg.payoutWorkers)
	)

	for _, e := range entries {
		entry := e
		if !entry.IsWinner || entry.PayoutMinor <= 0 {
			// Loser or zero-payout: mark skipped immediately.
			_ = s.repo.UpdateEntryStatus(ctx, entry.ID, domain.EntryStatusSkipped)
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			desc := fmt.Sprintf("Payout from market %s", settlement.MarketID)
			if err := s.wallet.Credit(
				context.Background(),
				entry.UserID.String(), settlement.Currency,
				entry.PayoutMinor,
				entry.IdempotencyKey, desc,
				settlement.MarketID.String(),
			); err != nil {
				s.log.Warn("wallet credit failed",
					zap.String("user_id", entry.UserID.String()),
					zap.Int64("payout", entry.PayoutMinor),
					zap.Error(err),
				)
				_ = s.repo.UpdateEntryStatus(context.Background(), entry.ID, domain.EntryStatusFailed)
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			_ = s.repo.UpdateEntryStatus(context.Background(), entry.ID, domain.EntryStatusPaid)
			s.log.Info("payout sent",
				zap.String("user_id", entry.UserID.String()),
				zap.Int64("amount", entry.PayoutMinor),
			)
		}()
	}

	wg.Wait()
	if failed > 0 {
		return fmt.Errorf("%d payout(s) failed", failed)
	}
	return nil
}

// collectInsuranceFee credits the insurance fund wallet.
func (s *SettlementService) collectInsuranceFee(ctx context.Context, settlement *domain.Settlement, fee int64) {
	idempotencyKey := fmt.Sprintf("insurance:%s", settlement.ID)
	desc := fmt.Sprintf("Insurance fee from market %s settlement", settlement.MarketID)
	if err := s.wallet.Credit(ctx,
		s.cfg.platformWalletID, settlement.Currency,
		fee,
		idempotencyKey, desc,
		settlement.MarketID.String(),
	); err != nil {
		s.log.Warn("insurance fee collection failed",
			zap.String("settlement_id", settlement.ID.String()),
			zap.Int64("fee", fee),
			zap.Error(err),
		)
	}
}

// runFraudCheck wraps the fraud check with panic recovery.
func (s *SettlementService) runFraudCheck(ctx context.Context, marketID uuid.UUID, outcomeIndex int, stakeMinor int64) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("fraud check panicked", zap.Any("panic", r))
		}
	}()
	s.fraud.CheckPosition(ctx, marketID, outcomeIndex, stakeMinor)
}
