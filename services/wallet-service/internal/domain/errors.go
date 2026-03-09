package domain

import "errors"

var (
	// ErrInsufficientFunds is returned when a debit exceeds available balance.
	ErrInsufficientFunds = errors.New("insufficient_funds")
	// ErrWalletNotFound is returned when no wallet exists for the given user+currency.
	ErrWalletNotFound = errors.New("wallet_not_found")
	// ErrWalletFrozen is returned when the wallet is inactive/frozen.
	ErrWalletFrozen = errors.New("wallet_frozen")
	// ErrDuplicateTxn is returned on idempotency key collision (safe to ignore).
	ErrDuplicateTxn = errors.New("duplicate_transaction")
	// ErrInvalidAmount is returned for zero or negative amounts.
	ErrInvalidAmount = errors.New("invalid_amount")
	// ErrUnsupportedCurrency is returned for unknown currency codes.
	ErrUnsupportedCurrency = errors.New("unsupported_currency")
)
