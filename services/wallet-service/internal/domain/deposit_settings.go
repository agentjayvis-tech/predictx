package domain

import (
	"time"

	"github.com/google/uuid"
)

// DepositSettings represents user deposit limit preferences
type DepositSettings struct {
	UserID                   uuid.UUID
	DailyDepositLimitMinor   int64
	MonthlyDepositLimitMinor *int64
	Enabled                  bool
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// Validate checks deposit settings for logical consistency
func (ds *DepositSettings) Validate() error {
	if ds.DailyDepositLimitMinor <= 0 {
		return ErrInvalidDepositLimit
	}
	if ds.MonthlyDepositLimitMinor != nil && *ds.MonthlyDepositLimitMinor <= 0 {
		return ErrInvalidDepositLimit
	}
	if ds.MonthlyDepositLimitMinor != nil && *ds.MonthlyDepositLimitMinor < ds.DailyDepositLimitMinor {
		return ErrMonthlyLimitBelowDaily
	}
	return nil
}

// DailyDepositTracking tracks user deposits for a given day
type DailyDepositTracking struct {
	UserID               uuid.UUID
	TrackedDate          time.Time
	TotalDepositedMinor  int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// CanDeposit checks if user can deposit the given amount
func (ddt *DailyDepositTracking) CanDeposit(amountMinor int64, dailyLimit int64) bool {
	return (ddt.TotalDepositedMinor + amountMinor) <= dailyLimit
}

// RemainingDailyBudget returns how much more the user can deposit today
func (ddt *DailyDepositTracking) RemainingDailyBudget(dailyLimit int64) int64 {
	remaining := dailyLimit - ddt.TotalDepositedMinor
	if remaining < 0 {
		return 0
	}
	return remaining
}
