package domain

import "testing"

func TestSupportedCurrencies(t *testing.T) {
	tests := []struct {
		currency Currency
		expected bool
	}{
		{CurrencyCoins, true},
		{CurrencyNGN, true},
		{CurrencyKES, true},
		{CurrencyPHP, true},
		{CurrencyUSDC, true},
		{Currency("UNKNOWN"), false},
	}

	for _, tt := range tests {
		if result := SupportedCurrencies[tt.currency]; result != tt.expected {
			t.Errorf("SupportedCurrencies[%s] = %v, want %v", tt.currency, result, tt.expected)
		}
	}
}

func TestTransactionTypeValues(t *testing.T) {
	tests := []TxnType{
		TxnTypeDeposit,
		TxnTypeSpend,
		TxnTypeRefund,
		TxnTypePayout,
		TxnTypeDailyReward,
		TxnTypeAdjustment,
	}

	for _, txnType := range tests {
		if string(txnType) == "" {
			t.Errorf("TxnType %v has empty string value", txnType)
		}
	}
}

func TestEntryTypeValues(t *testing.T) {
	if string(EntryTypeCredit) != "credit" {
		t.Errorf("EntryTypeCredit has wrong value: %s", EntryTypeCredit)
	}
	if string(EntryTypeDebit) != "debit" {
		t.Errorf("EntryTypeDebit has wrong value: %s", EntryTypeDebit)
	}
}

func TestFraudAlertTypeValues(t *testing.T) {
	tests := []FraudAlertType{
		FraudAlertRapidChanges,
		FraudAlertLargeCredit,
		FraudAlertRapidDrain,
	}

	for _, alertType := range tests {
		if string(alertType) == "" {
			t.Errorf("FraudAlertType %v has empty string value", alertType)
		}
	}
}
