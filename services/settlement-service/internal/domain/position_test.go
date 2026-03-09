package domain

import "testing"

func TestComputeInsuranceFee(t *testing.T) {
	tests := []struct {
		name       string
		pool       int64
		wantFee    int64
	}{
		{"1000 coins", 1000, 5},           // 0.5% = 5
		{"10000 coins", 10000, 50},        // 0.5% = 50
		{"100000 coins", 100000, 500},     // 0.5% = 500
		{"0 pool", 0, 0},
		{"1 coin (floor)", 1, 0},          // integer division floors
		{"200 coins", 200, 1},             // 200 * 50 / 10000 = 1
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeInsuranceFee(tt.pool)
			if got != tt.wantFee {
				t.Errorf("ComputeInsuranceFee(%d) = %d, want %d", tt.pool, got, tt.wantFee)
			}
		})
	}
}

func TestComputePayout(t *testing.T) {
	tests := []struct {
		name               string
		winnerStake        int64
		totalWinningStake  int64
		netPool            int64
		wantPayout         int64
	}{
		{"sole winner gets all", 100, 100, 190, 190},
		{"50/50 split", 100, 200, 190, 95},
		{"zero winning stake", 100, 0, 190, 0},
		{"proportional 1/4", 250, 1000, 2000, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputePayout(tt.winnerStake, tt.totalWinningStake, tt.netPool)
			if got != tt.wantPayout {
				t.Errorf("ComputePayout(%d, %d, %d) = %d, want %d",
					tt.winnerStake, tt.totalWinningStake, tt.netPool, got, tt.wantPayout)
			}
		})
	}
}

func TestDomainTypes(t *testing.T) {
	t.Run("PositionStatus constants", func(t *testing.T) {
		if PositionStatusOpen == "" || PositionStatusSettled == "" {
			t.Error("PositionStatus constants not defined")
		}
	})
	t.Run("SettlementStatus constants", func(t *testing.T) {
		if SettlementStatusPending == "" || SettlementStatusCompleted == "" {
			t.Error("SettlementStatus constants not defined")
		}
	})
	t.Run("EntryStatus constants", func(t *testing.T) {
		if EntryStatusPending == "" || EntryStatusPaid == "" {
			t.Error("EntryStatus constants not defined")
		}
	})
	t.Run("InsuranceFeeBps", func(t *testing.T) {
		if InsuranceFeeBps != 50 {
			t.Errorf("InsuranceFeeBps = %d, want 50", InsuranceFeeBps)
		}
	})
}
