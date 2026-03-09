package domain

import (
	"time"
)

// CountryRGPolicy defines responsible gambling policies per country/region
type CountryRGPolicy struct {
	CountryCode              string
	CoolOffCancellable       bool
	MaxDailyDepositLimitMinor *int64
	MaxCoolOffDurationHours  int
	CreatedAt                time.Time
	UpdatedAt                time.Time
}
