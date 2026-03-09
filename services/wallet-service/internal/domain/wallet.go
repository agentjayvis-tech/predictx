package domain

import (
	"time"

	"github.com/google/uuid"
)

// Currency is the ISO-style code for supported currencies.
type Currency string

const (
	CurrencyCoins Currency = "COINS" // Virtual currency — India F2P MVP
	CurrencyNGN   Currency = "NGN"   // Nigerian Naira
	CurrencyKES   Currency = "KES"   // Kenyan Shilling
	CurrencyPHP   Currency = "PHP"   // Philippine Peso
	CurrencyUSDC  Currency = "USDC"  // Circle USDC (on-chain)
)

// SupportedCurrencies lists all accepted currency codes.
var SupportedCurrencies = map[Currency]bool{
	CurrencyCoins: true,
	CurrencyNGN:   true,
	CurrencyKES:   true,
	CurrencyPHP:   true,
	CurrencyUSDC:  true,
}

// TxnType categorises the economic reason for a wallet transaction.
type TxnType string

const (
	TxnTypeDeposit     TxnType = "deposit"
	TxnTypeSpend       TxnType = "spend"
	TxnTypeRefund      TxnType = "refund"
	TxnTypePayout      TxnType = "payout"
	TxnTypeDailyReward TxnType = "daily_reward"
	TxnTypeAdjustment  TxnType = "adjustment"
)

// EntryType is the direction of a ledger entry from the user's perspective.
type EntryType string

const (
	EntryTypeCredit EntryType = "credit"
	EntryTypeDebit  EntryType = "debit"
)

// Wallet is a user's balance account for a single currency.
// balance_minor is authoritative and maintained atomically via apply_double_entry().
type Wallet struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Currency     Currency
	BalanceMinor int64 // amount in smallest currency unit (1 COIN = 1 minor unit)
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// FraudAlertType classifies the kind of suspicious behaviour detected.
type FraudAlertType string

const (
	FraudAlertRapidChanges FraudAlertType = "rapid_changes"
	FraudAlertLargeCredit  FraudAlertType = "large_credit"
	FraudAlertRapidDrain   FraudAlertType = "rapid_drain"
)

// FraudAlert records a suspicious event for investigation.
type FraudAlert struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	WalletID  uuid.UUID
	AlertType FraudAlertType
	Details   map[string]any
	Resolved  bool
	CreatedAt time.Time
}
