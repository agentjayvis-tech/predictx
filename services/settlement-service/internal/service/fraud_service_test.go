package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/predictx/settlement-service/internal/domain"
	"go.uber.org/zap"
)

// mockFraudRepo satisfies both FraudRepo and SettlementRepo for FraudService.
type mockFraudRepo struct {
	MockSettlementRepo
}

func newFraudService(t *testing.T) *FraudService {
	t.Helper()
	repo := NewMockSettlementRepo()
	return &FraudService{
		repo:                     repo,
		betWindowSecs:            60,
		concentrationThreshold:   3, // low threshold for test
		largeStakeThresholdMinor: 500,
		log:                      zap.NewNop(),
	}
}

func TestFraudService_LargeStake(t *testing.T) {
	repo := NewMockSettlementRepo()
	fraud := &FraudService{
		repo:                     repo,
		betWindowSecs:            60,
		concentrationThreshold:   50,
		largeStakeThresholdMinor: 500,
		log:                      zap.NewNop(),
	}

	ctx := context.Background()
	marketID := uuid.New()

	// No Redis — checkHighConcentration will silently fail.
	// Only checkLargeStake fires without Redis.
	fraud.checkLargeStake(ctx, marketID, 1, 1000)

	if len(repo.alerts) != 1 {
		t.Errorf("expected 1 fraud alert, got %d", len(repo.alerts))
	}
	if repo.alerts[0].Severity != "medium" {
		t.Errorf("expected medium severity, got %s", repo.alerts[0].Severity)
	}
}

func TestFraudService_NormalStake(t *testing.T) {
	repo := NewMockSettlementRepo()
	fraud := &FraudService{
		repo:                     repo,
		betWindowSecs:            60,
		concentrationThreshold:   50,
		largeStakeThresholdMinor: 500,
		log:                      zap.NewNop(),
	}

	ctx := context.Background()
	marketID := uuid.New()

	fraud.checkLargeStake(ctx, marketID, 1, 100) // below threshold
	if len(repo.alerts) != 0 {
		t.Errorf("expected 0 fraud alerts, got %d", len(repo.alerts))
	}
}

func TestFraudAlert_Persist(t *testing.T) {
	repo := NewMockSettlementRepo()
	fraud := &FraudService{
		repo:                     repo,
		betWindowSecs:            60,
		concentrationThreshold:   50,
		largeStakeThresholdMinor: 100,
		log:                      zap.NewNop(),
	}

	ctx := context.Background()
	marketID := uuid.New()

	fraud.alert(ctx, marketID, 0, "test alert", "low")

	if len(repo.alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(repo.alerts))
	}
	alert := repo.alerts[0]
	if alert.MarketID != marketID {
		t.Errorf("alert market_id = %v, want %v", alert.MarketID, marketID)
	}
	if alert.Severity != "low" {
		t.Errorf("alert severity = %s, want low", alert.Severity)
	}
	if alert.ID == (uuid.UUID{}) {
		t.Error("alert ID should not be zero")
	}
}

func TestComputeInsuranceFee_InService(t *testing.T) {
	fee := domain.ComputeInsuranceFee(10000)
	if fee != 50 {
		t.Errorf("ComputeInsuranceFee(10000) = %d, want 50", fee)
	}
}
