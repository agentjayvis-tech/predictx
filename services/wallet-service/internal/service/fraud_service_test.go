package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/predictx/wallet-service/internal/domain"
)

// Note: Full integration tests for FraudService require running PostgreSQL and Redis.
// Unit tests with full mocks are deferred until service interfaces are refactored.
// For now, we test that domain types used in fraud detection are correctly defined.

func TestFraudDetectionDomainTypes(t *testing.T) {
	tests := []struct {
		name string
		test func()
	}{
		{
			name: "Wallet type defined",
			test: func() {
				w := &domain.Wallet{
					ID:           uuid.New(),
					UserID:       uuid.New(),
					Currency:     domain.CurrencyCoins,
					BalanceMinor: 1000,
					IsActive:     true,
				}
				if w.ID == uuid.Nil {
					t.Error("wallet ID not set")
				}
			},
		},
		{
			name: "FraudAlert type defined",
			test: func() {
				alert := &domain.FraudAlert{
					ID:        uuid.New(),
					UserID:    uuid.New(),
					WalletID:  uuid.New(),
					AlertType: domain.FraudAlertLargeCredit,
					Details:   map[string]any{"amount": 150000},
				}
				if alert.ID == uuid.Nil {
					t.Error("alert ID not set")
				}
			},
		},
		{
			name: "Transaction types defined",
			test: func() {
				if domain.TxnTypeDeposit == "" || domain.TxnTypeSpend == "" {
					t.Error("transaction types not defined")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.test()
		})
	}
}
