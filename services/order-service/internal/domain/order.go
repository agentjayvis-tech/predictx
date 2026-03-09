package domain

import (
	"time"

	"github.com/google/uuid"
)

// OrderType represents the direction of an order: buy or sell.
type OrderType string

const (
	OrderTypeBuy  OrderType = "buy"
	OrderTypeSell OrderType = "sell"
)

var SupportedOrderTypes = map[OrderType]bool{
	OrderTypeBuy:  true,
	OrderTypeSell: true,
}

// OrderStatus represents the lifecycle state of an order.
type OrderStatus string

const (
	StatusPending   OrderStatus = "pending"
	StatusMatched   OrderStatus = "matched"
	StatusSettled   OrderStatus = "settled"
	StatusCancelled OrderStatus = "cancelled"
	StatusFailed    OrderStatus = "failed"
)

var SupportedOrderStatuses = map[OrderStatus]bool{
	StatusPending:   true,
	StatusMatched:   true,
	StatusSettled:   true,
	StatusCancelled: true,
	StatusFailed:    true,
}

// TimeInForce represents order execution strategy.
type TimeInForce string

const (
	TimeInForceIOC TimeInForce = "ioc" // Immediate-or-cancel
	TimeInForceGTC TimeInForce = "gtc" // Good-till-cancelled
)

var SupportedTimeInForce = map[TimeInForce]bool{
	TimeInForceIOC: true,
	TimeInForceGTC: true,
}

// Order represents a user's placement bet on a market outcome.
type Order struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	MarketID        uuid.UUID
	OrderType       OrderType
	Status          OrderStatus
	TimeInForce     TimeInForce
	PriceMinor      int64  // Bet amount in minor units (e.g., 100 coins)
	QuantityShares  int64  // For scalar markets; default 1 for binary
	Currency        string // "COINS", "NGN", etc.
	OutcomeIndex    int    // 0/1 for binary, index for scalar
	IdempotencyKey  string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// RGLimit tracks daily/weekly spending for responsible gambling enforcement.
type RGLimit struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	DailySpentMinor   int64
	WeeklySpentMinor  int64
	DailyResetAt      time.Time
	WeeklyResetAt     time.Time
	UpdatedAt         time.Time
}

// ValidStatusTransitions defines allowed state machine transitions.
var ValidStatusTransitions = map[OrderStatus][]OrderStatus{
	StatusPending:   {StatusMatched, StatusCancelled, StatusFailed},
	StatusMatched:   {StatusSettled, StatusFailed},
	StatusSettled:   {},
	StatusCancelled: {},
	StatusFailed:    {},
}

// IsValidTransition checks if transitioning from current to next status is allowed.
func IsValidTransition(current, next OrderStatus) bool {
	allowed, ok := ValidStatusTransitions[current]
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
