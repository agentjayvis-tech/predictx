package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/predictx/settlement-service/internal/domain"
	"go.uber.org/zap"
)

func newTestService(t *testing.T) (*SettlementService, *MockSettlementRepo, *MockWalletClient, *MockSettlementPublisher) {
	t.Helper()
	repo := NewMockSettlementRepo()
	wallet := NewMockWalletClient()
	pub := NewMockSettlementPublisher()
	fraud := &FraudService{
		repo:                     repo,
		betWindowSecs:            60,
		concentrationThreshold:   50,
		largeStakeThresholdMinor: 10_000_00,
		log:                      zap.NewNop(),
	}
	svc := NewSettlementService(repo, wallet, pub, fraud, "platform-wallet-id", "COINS", 5, zap.NewNop())
	return svc, repo, wallet, pub
}

func TestRecordPosition(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	marketID := uuid.New()

	pos, err := svc.RecordPosition(ctx, userID, marketID, 1, 100, "COINS")
	if err != nil {
		t.Fatalf("RecordPosition() error = %v", err)
	}
	if pos.StakeMinor != 100 {
		t.Errorf("StakeMinor = %d, want 100", pos.StakeMinor)
	}
}

func TestRecordPosition_Aggregates(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()

	userID := uuid.New()
	marketID := uuid.New()

	svc.RecordPosition(ctx, userID, marketID, 1, 100, "COINS") //nolint:errcheck
	pos, err := svc.RecordPosition(ctx, userID, marketID, 1, 200, "COINS")
	if err != nil {
		t.Fatalf("second RecordPosition() error = %v", err)
	}
	if pos.StakeMinor != 300 {
		t.Errorf("aggregated StakeMinor = %d, want 300", pos.StakeMinor)
	}
}

func TestSettleMarket_BasicPayout(t *testing.T) {
	svc, repo, wallet, pub := newTestService(t)
	ctx := context.Background()

	marketID := uuid.New()
	winnerID := uuid.New()
	loserID := uuid.New()

	// Winner bets 100 on outcome 1; loser bets 100 on outcome 0.
	repo.UpsertPosition(ctx, &domain.Position{ //nolint:errcheck
		ID: uuid.New(), UserID: winnerID, MarketID: marketID,
		OutcomeIndex: 1, StakeMinor: 100, Currency: "COINS", Status: domain.PositionStatusOpen,
	})
	repo.UpsertPosition(ctx, &domain.Position{ //nolint:errcheck
		ID: uuid.New(), UserID: loserID, MarketID: marketID,
		OutcomeIndex: 0, StakeMinor: 100, Currency: "COINS", Status: domain.PositionStatusOpen,
	})

	if err := svc.SettleMarket(ctx, marketID, 1, "res-001"); err != nil {
		t.Fatalf("SettleMarket() error = %v", err)
	}

	// Insurance fee = 200 * 50 / 10000 = 1 coin
	// Net pool = 199
	// Winner gets 199 (sole winner)
	if wallet.CreditCount() < 1 {
		t.Error("expected at least 1 wallet credit for winner payout")
	}

	total := wallet.TotalCredited()
	// 199 payout + possible 1 insurance fee credit
	if total < 199 {
		t.Errorf("total credited = %d, want >= 199", total)
	}

	if pub.CompletedCount() == 0 {
		// Publisher is async; wait briefly
		time.Sleep(50 * time.Millisecond)
	}
}

func TestSettleMarket_Idempotent(t *testing.T) {
	svc, repo, wallet, _ := newTestService(t)
	ctx := context.Background()

	marketID := uuid.New()
	winnerID := uuid.New()

	repo.UpsertPosition(ctx, &domain.Position{ //nolint:errcheck
		ID: uuid.New(), UserID: winnerID, MarketID: marketID,
		OutcomeIndex: 1, StakeMinor: 100, Currency: "COINS", Status: domain.PositionStatusOpen,
	})

	if err := svc.SettleMarket(ctx, marketID, 1, "res-001"); err != nil {
		t.Fatalf("first SettleMarket() error = %v", err)
	}

	// Mark completed in repo.
	s, _ := repo.GetSettlementByMarket(ctx, marketID)
	if s != nil {
		repo.UpdateSettlementStatus(ctx, s.ID, domain.SettlementStatusCompleted) //nolint:errcheck
	}

	firstCount := wallet.CreditCount()

	// Second call must be a no-op.
	if err := svc.SettleMarket(ctx, marketID, 1, "res-001"); err != nil {
		t.Fatalf("second SettleMarket() error = %v", err)
	}
	if wallet.CreditCount() != firstCount {
		t.Errorf("second SettleMarket made %d extra wallet calls, expected 0", wallet.CreditCount()-firstCount)
	}
}

func TestSettleMarket_NoPositions(t *testing.T) {
	svc, _, wallet, _ := newTestService(t)
	ctx := context.Background()

	marketID := uuid.New()
	if err := svc.SettleMarket(ctx, marketID, 1, "res-001"); err != nil {
		t.Fatalf("SettleMarket with no positions error = %v", err)
	}
	if wallet.CreditCount() != 0 {
		t.Errorf("expected 0 wallet calls, got %d", wallet.CreditCount())
	}
}

func TestRefundMarket(t *testing.T) {
	svc, repo, wallet, pub := newTestService(t)
	ctx := context.Background()

	marketID := uuid.New()
	users := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	for i, u := range users {
		repo.UpsertPosition(ctx, &domain.Position{ //nolint:errcheck
			ID: uuid.New(), UserID: u, MarketID: marketID,
			OutcomeIndex: i % 2, StakeMinor: int64((i + 1) * 100), Currency: "COINS",
			Status: domain.PositionStatusOpen,
		})
	}

	if err := svc.RefundMarket(ctx, marketID, "market_voided"); err != nil {
		t.Fatalf("RefundMarket() error = %v", err)
	}

	if wallet.CreditCount() != 3 {
		t.Errorf("expected 3 refund credits, got %d", wallet.CreditCount())
	}
	// Wait for async publish.
	time.Sleep(20 * time.Millisecond)
	_ = pub
}

func TestGetUserPnL_Winner(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	ctx := context.Background()

	marketID := uuid.New()
	winnerID := uuid.New()
	loserID := uuid.New()

	repo.UpsertPosition(ctx, &domain.Position{ //nolint:errcheck
		ID: uuid.New(), UserID: winnerID, MarketID: marketID,
		OutcomeIndex: 1, StakeMinor: 1000, Currency: "COINS", Status: domain.PositionStatusSettled,
	})
	repo.UpsertPosition(ctx, &domain.Position{ //nolint:errcheck
		ID: uuid.New(), UserID: loserID, MarketID: marketID,
		OutcomeIndex: 0, StakeMinor: 1000, Currency: "COINS", Status: domain.PositionStatusSettled,
	})

	// Manually create a completed settlement.
	repo.CreateSettlement(ctx, &domain.Settlement{ //nolint:errcheck
		ID: uuid.New(), MarketID: marketID, ResolutionID: "res-test",
		Status:            domain.SettlementStatusCompleted,
		WinningOutcome:    1,
		TotalPoolMinor:    2000,
		InsuranceFeeMinor: 10,
		NetPoolMinor:      1990,
		WinningStakeMinor: 1000,
		WinnerCount: 1, LoserCount: 1, Currency: "COINS",
	})

	pnl, err := svc.GetUserPnL(ctx, marketID, winnerID)
	if err != nil {
		t.Fatalf("GetUserPnL() error = %v", err)
	}
	// Winner: payout = 1000/1000 * 1990 = 1990; PnL = 1990 - 1000 = 990
	if pnl != 990 {
		t.Errorf("winner PnL = %d, want 990", pnl)
	}

	loserPnl, err := svc.GetUserPnL(ctx, marketID, loserID)
	if err != nil {
		t.Fatalf("GetUserPnL() loser error = %v", err)
	}
	// Loser: payout = 0; PnL = -1000
	if loserPnl != -1000 {
		t.Errorf("loser PnL = %d, want -1000", loserPnl)
	}
}
