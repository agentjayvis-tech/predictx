package domain

import (
	"time"

	"github.com/google/uuid"
)

// MarketCategory represents the category of a prediction market.
type MarketCategory string

const (
	CategorySports        MarketCategory = "sports"
	CategoryEntertainment MarketCategory = "entertainment"
	CategoryPolitics      MarketCategory = "politics"
	CategoryWeather       MarketCategory = "weather"
	CategoryFinance       MarketCategory = "finance"
	CategoryTrending      MarketCategory = "trending"
	CategoryLocal         MarketCategory = "local"
)

// SupportedCategories is used for validation.
var SupportedCategories = map[MarketCategory]bool{
	CategorySports:        true,
	CategoryEntertainment: true,
	CategoryPolitics:      true,
	CategoryWeather:       true,
	CategoryFinance:       true,
	CategoryTrending:      true,
	CategoryLocal:         true,
}

// MarketStatus represents the lifecycle state of a market.
type MarketStatus string

const (
	StatusDraft             MarketStatus = "draft"
	StatusActive            MarketStatus = "active"
	StatusSuspended         MarketStatus = "suspended"
	StatusPendingResolution MarketStatus = "pending_resolution"
	StatusResolved          MarketStatus = "resolved"
	StatusVoided            MarketStatus = "voided"
	StatusArchived          MarketStatus = "archived"
)

// CreatorType distinguishes admin-curated from user-proposed markets.
type CreatorType string

const (
	CreatorAdmin CreatorType = "admin"
	CreatorUser  CreatorType = "user"
)

// ValidTransitions defines the allowed state machine transitions.
var ValidTransitions = map[MarketStatus][]MarketStatus{
	StatusDraft:             {StatusActive, StatusVoided},
	StatusActive:            {StatusSuspended, StatusPendingResolution, StatusVoided},
	StatusSuspended:         {StatusActive, StatusVoided},
	StatusPendingResolution: {StatusResolved, StatusVoided},
	StatusResolved:          {StatusArchived},
	StatusVoided:            {StatusArchived},
}

// IsValidTransition returns true if transitioning from current to next is allowed.
func IsValidTransition(current, next MarketStatus) bool {
	allowed, ok := ValidTransitions[current]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
}

// Market represents a prediction market.
type Market struct {
	ID                 uuid.UUID
	Title              string
	Question           string
	ResolutionCriteria string
	Category           MarketCategory
	Status             MarketStatus
	CreatorID          uuid.UUID
	CreatorType        CreatorType
	PoolAmountMinor    int64          // updated by Matching Engine / Settlement
	Currency           string         // default "COINS"
	ClosesAt           time.Time
	ResolvesAt         *time.Time     // nil → uses ClosesAt
	Metadata           map[string]any // resolver-specific context (sport, league, etc.)
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CreateMarketRequest is the input for creating a new market.
type CreateMarketRequest struct {
	Title              string
	Question           string
	ResolutionCriteria string
	Category           MarketCategory
	CreatorID          uuid.UUID
	CreatorType        CreatorType
	ClosesAt           time.Time
	ResolvesAt         *time.Time
	Metadata           map[string]any
}

// ListFilters controls the listing query.
type ListFilters struct {
	Status   MarketStatus
	Category MarketCategory
	Limit    int
	Offset   int
}
