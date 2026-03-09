package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/predictx/order-service/internal/domain"
	"github.com/predictx/order-service/internal/repository"
)

// RGService enforces responsible gambling limits (daily/weekly spending caps).
type RGService struct {
	repo              *repository.OrderRepo
	dailyLimitCoins   int64
	weeklyLimitCoins  int64
}

// NewRGService creates a new RG service.
func NewRGService(repo *repository.OrderRepo, dailyLimit, weeklyLimit int64) *RGService {
	return &RGService{
		repo:             repo,
		dailyLimitCoins:  dailyLimit,
		weeklyLimitCoins: weeklyLimit,
	}
}

// CheckAndUpdate verifies RG limits and updates spending if order is created.
// Returns (withinDaily, withinWeekly, dailyRemaining, weeklyRemaining, error).
// If either limit is exceeded, an error is returned.
func (svc *RGService) CheckAndUpdate(ctx context.Context, userID uuid.UUID, additionalSpendMinor int64) (bool, bool, int64, int64, error) {
	limit, err := svc.repo.GetRGLimit(ctx, userID)
	if err != nil {
		return false, false, 0, 0, err
	}

	// First-time user: create limit record
	if limit == nil {
		limit = &domain.RGLimit{
			UserID:        userID,
			DailySpentMinor:   0,
			WeeklySpentMinor:  0,
			DailyResetAt:   time.Now(),
			WeeklyResetAt:  mondayMidnight(time.Now()),
			UpdatedAt:      time.Now(),
		}
		if _, err := svc.repo.CreateRGLimit(ctx, limit); err != nil {
			return false, false, 0, 0, err
		}
	}

	// Reset daily if crossed midnight
	now := time.Now()
	if limit.DailyResetAt.Before(now.Truncate(24 * time.Hour)) {
		limit.DailySpentMinor = 0
		limit.DailyResetAt = now
	}

	// Reset weekly if crossed Monday midnight
	if limit.WeeklyResetAt.Before(mondayMidnight(now)) {
		limit.WeeklySpentMinor = 0
		limit.WeeklyResetAt = now
	}

	// Check against limits
	newDaily := limit.DailySpentMinor + additionalSpendMinor
	newWeekly := limit.WeeklySpentMinor + additionalSpendMinor

	withinDaily := newDaily <= svc.dailyLimitCoins
	withinWeekly := newWeekly <= svc.weeklyLimitCoins

	dailyRemaining := svc.dailyLimitCoins - newDaily
	if dailyRemaining < 0 {
		dailyRemaining = 0
	}

	weeklyRemaining := svc.weeklyLimitCoins - newWeekly
	if weeklyRemaining < 0 {
		weeklyRemaining = 0
	}

	// If limits violated, return error without updating DB
	if !withinDaily {
		return false, withinWeekly, dailyRemaining, weeklyRemaining, domain.ErrRGDailyLimitExceeded
	}
	if !withinWeekly {
		return true, false, dailyRemaining, weeklyRemaining, domain.ErrRGWeeklyLimitExceeded
	}

	// Update DB with new spending
	limit.DailySpentMinor = newDaily
	limit.WeeklySpentMinor = newWeekly
	limit.UpdatedAt = now

	if err := svc.repo.UpdateRGLimit(ctx, limit); err != nil {
		return true, true, dailyRemaining, weeklyRemaining, err
	}

	return true, true, dailyRemaining, weeklyRemaining, nil
}

// GetLimits returns current RG limit status without updating.
func (svc *RGService) GetLimits(ctx context.Context, userID uuid.UUID) (int64, int64, error) {
	limit, err := svc.repo.GetRGLimit(ctx, userID)
	if err != nil {
		return 0, 0, err
	}

	// First-time user
	if limit == nil {
		return svc.dailyLimitCoins, svc.weeklyLimitCoins, nil
	}

	now := time.Now()

	// Reset daily if crossed midnight
	if limit.DailyResetAt.Before(now.Truncate(24 * time.Hour)) {
		limit.DailySpentMinor = 0
	}

	// Reset weekly if crossed Monday midnight
	if limit.WeeklyResetAt.Before(mondayMidnight(now)) {
		limit.WeeklySpentMinor = 0
	}

	dailyRemaining := svc.dailyLimitCoins - limit.DailySpentMinor
	if dailyRemaining < 0 {
		dailyRemaining = 0
	}

	weeklyRemaining := svc.weeklyLimitCoins - limit.WeeklySpentMinor
	if weeklyRemaining < 0 {
		weeklyRemaining = 0
	}

	return dailyRemaining, weeklyRemaining, nil
}

// mondayMidnight returns the start of the week (Monday at 00:00:00) in the given time's timezone.
func mondayMidnight(t time.Time) time.Time {
	// Calculate days since Monday (0=Monday)
	daysSinceMonday := int(t.Weekday() - time.Monday)
	if daysSinceMonday < 0 {
		daysSinceMonday += 7 // Handle Sunday case
	}

	// Truncate to midnight
	midnight := t.Truncate(24 * time.Hour)

	// Subtract days to get to Monday
	return midnight.AddDate(0, 0, -daysSinceMonday)
}
