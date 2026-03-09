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

	// Responsible Gambling errors
	// ErrDepositLimitExceeded is returned when user deposit exceeds daily/monthly limit.
	ErrDepositLimitExceeded = errors.New("deposit_limit_exceeded")
	// ErrInCoolOffPeriod is returned when user is in an active cool-off period.
	ErrInCoolOffPeriod = errors.New("in_cool_off_period")
	// ErrSelfExcluded is returned when user is self-excluded.
	ErrSelfExcluded = errors.New("self_excluded")
	// ErrInvalidDepositLimit is returned for invalid deposit limit values.
	ErrInvalidDepositLimit = errors.New("invalid_deposit_limit")
	// ErrMonthlyLimitBelowDaily is returned when monthly limit < daily limit.
	ErrMonthlyLimitBelowDaily = errors.New("monthly_limit_below_daily")
	// ErrInvalidCoolOff is returned for invalid cool-off configuration.
	ErrInvalidCoolOff = errors.New("invalid_cool_off")
	// ErrInvalidCoolOffDuration is returned for unsupported cool-off duration.
	ErrInvalidCoolOffDuration = errors.New("invalid_cool_off_duration")
	// ErrInvalidSelfExclusion is returned for invalid self-exclusion configuration.
	ErrInvalidSelfExclusion = errors.New("invalid_self_exclusion")
	// ErrCoolOffNotCancellable is returned when user's region doesn't allow cancellation.
	ErrCoolOffNotCancellable = errors.New("cool_off_not_cancellable_in_region")
)
