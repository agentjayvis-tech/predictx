package domain

import (
	"time"

	"github.com/google/uuid"
)

// ExclusionSettings represents user cool-off and self-exclusion preferences
type ExclusionSettings struct {
	UserID               uuid.UUID
	CountryCode          *string
	CoolOffUntil         *time.Time  // NULL = not in cool-off
	CoolOffDurationHours *int        // 24, 168 (7d), 720 (30d)
	CoolOffCancellable   bool
	IsSelfExcluded       bool
	SelfExcludedAt       *time.Time
	SelfExclusionDurationDays *int   // NULL = permanent
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// IsInCoolOff checks if user is currently in a cool-off period
func (es *ExclusionSettings) IsInCoolOff(now time.Time) bool {
	if es.CoolOffUntil == nil {
		return false
	}
	return now.Before(*es.CoolOffUntil)
}

// CoolOffRemaining returns duration remaining in cool-off period
func (es *ExclusionSettings) CoolOffRemaining(now time.Time) time.Duration {
	if !es.IsInCoolOff(now) {
		return 0
	}
	return es.CoolOffUntil.Sub(now)
}

// Validate checks exclusion settings for logical consistency
func (es *ExclusionSettings) Validate() error {
	if es.IsSelfExcluded && es.SelfExcludedAt == nil {
		return ErrInvalidSelfExclusion
	}
	if es.CoolOffUntil != nil && es.CoolOffDurationHours == nil {
		return ErrInvalidCoolOff
	}
	if es.CoolOffDurationHours != nil {
		// Validate cool-off duration is one of: 24h, 7d (168h), 30d (720h)
		if *es.CoolOffDurationHours != 24 && *es.CoolOffDurationHours != 168 && *es.CoolOffDurationHours != 720 {
			return ErrInvalidCoolOffDuration
		}
	}
	return nil
}

// SelfExclusionRemaining returns days remaining until self-exclusion expires (or nil if permanent)
func (es *ExclusionSettings) SelfExclusionRemaining(now time.Time) *time.Duration {
	if !es.IsSelfExcluded || es.SelfExcludedAt == nil {
		return nil
	}
	if es.SelfExclusionDurationDays == nil {
		// Permanent self-exclusion
		return nil
	}
	expiresAt := es.SelfExcludedAt.AddDate(0, 0, *es.SelfExclusionDurationDays)
	if now.After(expiresAt) {
		return nil
	}
	remaining := expiresAt.Sub(now)
	return &remaining
}

// IsPermanentlySelfExcluded checks if self-exclusion is permanent
func (es *ExclusionSettings) IsPermanentlySelfExcluded() bool {
	return es.IsSelfExcluded && es.SelfExclusionDurationDays == nil
}
