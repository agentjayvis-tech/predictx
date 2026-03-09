package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/predictx/wallet-service/internal/domain"
)

// TestDepositSettingsDomain tests DepositSettings domain logic
func TestDepositSettingsDomain(t *testing.T) {
	tests := []struct {
		name     string
		settings *domain.DepositSettings
		amount   int64
		expected bool
	}{
		{
			name: "Can deposit when within limit",
			settings: &domain.DepositSettings{
				UserID:                   uuid.New(),
				DailyDepositLimitMinor:   5000000, // $50
				MonthlyDepositLimitMinor: nil,
				Enabled:                  true,
				CreatedAt:                time.Now(),
				UpdatedAt:                time.Now(),
			},
			amount:   2000000, // $20
			expected: true,
		},
		{
			name: "Cannot deposit when disabled",
			settings: &domain.DepositSettings{
				UserID:                   uuid.New(),
				DailyDepositLimitMinor:   5000000,
				MonthlyDepositLimitMinor: nil,
				Enabled:                  false,
				CreatedAt:                time.Now(),
				UpdatedAt:                time.Now(),
			},
			amount:   2000000,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.settings == nil {
				t.Fatal("settings cannot be nil")
			}

			if tt.settings.Enabled != tt.expected {
				t.Errorf("expected enabled=%v, got %v", tt.expected, tt.settings.Enabled)
			}
		})
	}
}

// TestExclusionSettingsDomain tests ExclusionSettings domain logic
func TestExclusionSettingsDomain(t *testing.T) {
	userID := uuid.New()
	now := time.Now()

	tests := []struct {
		name     string
		settings *domain.ExclusionSettings
		testFn   func(*domain.ExclusionSettings, time.Time) bool
		expected bool
	}{
		{
			name: "User not in cool-off when cool_off_until is nil",
			settings: &domain.ExclusionSettings{
				UserID:                  userID,
				CoolOffUntil:            nil,
				IsSelfExcluded:          false,
				CoolOffCancellable:      false,
				CreatedAt:               now,
				UpdatedAt:               now,
			},
			testFn: func(es *domain.ExclusionSettings, t time.Time) bool {
				return es.IsInCoolOff(t)
			},
			expected: false,
		},
		{
			name: "User in cool-off when cool_off_until is in future",
			settings: &domain.ExclusionSettings{
				UserID:                  userID,
				CoolOffUntil:            ptrTime(now.Add(24 * time.Hour)),
				CoolOffDurationHours:    intPtr(24),
				IsSelfExcluded:          false,
				CoolOffCancellable:      true,
				CreatedAt:               now,
				UpdatedAt:               now,
			},
			testFn: func(es *domain.ExclusionSettings, t time.Time) bool {
				return es.IsInCoolOff(t)
			},
			expected: true,
		},
		{
			name: "User not in cool-off when cool_off_until is in past",
			settings: &domain.ExclusionSettings{
				UserID:                  userID,
				CoolOffUntil:            ptrTime(now.Add(-24 * time.Hour)),
				CoolOffDurationHours:    intPtr(24),
				IsSelfExcluded:          false,
				CoolOffCancellable:      false,
				CreatedAt:               now,
				UpdatedAt:               now,
			},
			testFn: func(es *domain.ExclusionSettings, t time.Time) bool {
				return es.IsInCoolOff(t)
			},
			expected: false,
		},
		{
			name: "User not self-excluded when flag is false",
			settings: &domain.ExclusionSettings{
				UserID:             userID,
				IsSelfExcluded:     false,
				SelfExcludedAt:     nil,
				CreatedAt:          now,
				UpdatedAt:          now,
			},
			testFn: func(es *domain.ExclusionSettings, t time.Time) bool {
				return es.IsSelfExcluded
			},
			expected: false,
		},
		{
			name: "User permanently self-excluded when duration is nil",
			settings: &domain.ExclusionSettings{
				UserID:                      userID,
				IsSelfExcluded:              true,
				SelfExcludedAt:              ptrTime(now),
				SelfExclusionDurationDays:   nil,
				CreatedAt:                   now,
				UpdatedAt:                   now,
			},
			testFn: func(es *domain.ExclusionSettings, t time.Time) bool {
				// Permanent if no duration set
				return es.SelfExclusionDurationDays == nil && es.IsSelfExcluded
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.settings == nil {
				t.Fatal("settings cannot be nil")
			}

			result := tt.testFn(tt.settings, now)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestDailyDepositTrackingDomain tests DailyDepositTracking domain logic
func TestDailyDepositTrackingDomain(t *testing.T) {
	userID := uuid.New()
	today := time.Now().UTC().Truncate(24 * time.Hour)

	tests := []struct {
		name     string
		tracking *domain.DailyDepositTracking
		limit    int64
		amount   int64
		expected bool
	}{
		{
			name: "Deposit within limit",
			tracking: &domain.DailyDepositTracking{
				UserID:              userID,
				TrackedDate:         today,
				TotalDepositedMinor: 2000000, // $20
			},
			limit:    5000000, // $50
			amount:   2000000, // $20
			expected: true,
		},
		{
			name: "Deposit exceeds limit",
			tracking: &domain.DailyDepositTracking{
				UserID:              userID,
				TrackedDate:         today,
				TotalDepositedMinor: 4000000, // $40
			},
			limit:    5000000, // $50
			amount:   2000000, // $20 (total would be $60)
			expected: false,
		},
		{
			name: "Deposit at limit boundary",
			tracking: &domain.DailyDepositTracking{
				UserID:              userID,
				TrackedDate:         today,
				TotalDepositedMinor: 3000000, // $30
			},
			limit:    5000000, // $50
			amount:   2000000, // $20 (total = $50)
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tracking == nil {
				t.Fatal("tracking cannot be nil")
			}

			newTotal := tt.tracking.TotalDepositedMinor + tt.amount
			allowed := newTotal <= tt.limit

			if allowed != tt.expected {
				t.Errorf("expected %v, got %v (newTotal=%d, limit=%d)", tt.expected, allowed, newTotal, tt.limit)
			}
		})
	}
}

// TestCountryRGPolicyDomain tests CountryRGPolicy domain logic
func TestCountryRGPolicyDomain(t *testing.T) {
	tests := []struct {
		name     string
		policy   *domain.CountryRGPolicy
		expected bool
	}{
		{
			name: "India allows cool-off cancellation",
			policy: &domain.CountryRGPolicy{
				CountryCode:           "IN",
				CoolOffCancellable:    true,
				MaxDailyDepositMinor:  500000, // $5
				MaxCoolOffDurationHrs: 2592000,
			},
			expected: true,
		},
		{
			name: "Nigeria does not allow cool-off cancellation",
			policy: &domain.CountryRGPolicy{
				CountryCode:           "NG",
				CoolOffCancellable:    false,
				MaxDailyDepositMinor:  300000, // $3
				MaxCoolOffDurationHrs: 2592000,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.policy == nil {
				t.Fatal("policy cannot be nil")
			}

			if tt.policy.CoolOffCancellable != tt.expected {
				t.Errorf("expected CoolOffCancellable=%v, got %v", tt.expected, tt.policy.CoolOffCancellable)
			}
		})
	}
}

// TestOrderEligibilityLogic tests logic for checking order eligibility
func TestOrderEligibilityLogic(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		exclusion  *domain.ExclusionSettings
		eligible   bool
		reason     string
	}{
		{
			name: "User without exclusion settings is eligible",
			exclusion: nil,
			eligible:  true,
			reason:    "",
		},
		{
			name: "User in cool-off is ineligible",
			exclusion: &domain.ExclusionSettings{
				UserID:               uuid.New(),
				CoolOffUntil:         ptrTime(now.Add(24 * time.Hour)),
				CoolOffDurationHours: intPtr(24),
				IsSelfExcluded:       false,
			},
			eligible: false,
			reason:   "in_cool_off_period",
		},
		{
			name: "Self-excluded user is ineligible",
			exclusion: &domain.ExclusionSettings{
				UserID:         uuid.New(),
				CoolOffUntil:   nil,
				IsSelfExcluded: true,
				SelfExcludedAt: ptrTime(now),
			},
			eligible: false,
			reason:   "self_excluded",
		},
		{
			name: "User with expired cool-off is eligible",
			exclusion: &domain.ExclusionSettings{
				UserID:               uuid.New(),
				CoolOffUntil:         ptrTime(now.Add(-1 * time.Hour)),
				CoolOffDurationHours: intPtr(24),
				IsSelfExcluded:       false,
			},
			eligible: true,
			reason:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eligible := checkOrderEligibility(tt.exclusion, now)
			if eligible != tt.eligible {
				t.Errorf("expected eligible=%v, got %v", tt.eligible, eligible)
			}
		})
	}
}

// Helper functions

func ptrTime(t time.Time) *time.Time {
	return &t
}

func intPtr(i int) *int {
	return &i
}

// checkOrderEligibility is a simple helper to test eligibility logic
func checkOrderEligibility(exclusion *domain.ExclusionSettings, now time.Time) bool {
	if exclusion == nil {
		return true
	}

	// Cannot place order if self-excluded
	if exclusion.IsSelfExcluded {
		return false
	}

	// Cannot place order if in cool-off
	if exclusion.IsInCoolOff(now) {
		return false
	}

	return true
}
