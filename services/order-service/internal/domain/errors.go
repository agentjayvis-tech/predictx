package domain

import "errors"

// Domain sentinel errors for business logic failures.
var (
	ErrInvalidMarket           = errors.New("market does not exist or is not active")
	ErrInsufficientBalance     = errors.New("insufficient balance for order")
	ErrOutcomeIndexOutOfBounds = errors.New("outcome_index outside market valid outcomes")
	ErrMarketClosed            = errors.New("market has closed for new orders")
	ErrRGDailyLimitExceeded    = errors.New("daily responsible gambling limit exceeded")
	ErrRGWeeklyLimitExceeded   = errors.New("weekly responsible gambling limit exceeded")
	ErrRateLimitExceeded       = errors.New("rate limit exceeded (100 orders/min)")
	ErrInvalidTransition       = errors.New("order status transition not allowed")
	ErrDuplicateOrder          = errors.New("order with this idempotency key already exists")
	ErrOrderNotFound           = errors.New("order not found")
	ErrUnsupportedOrderType    = errors.New("unsupported order type")
	ErrUnsupportedTimeInForce  = errors.New("unsupported time in force")
	ErrInvalidAmount           = errors.New("invalid order amount")
	ErrInvalidQuantity         = errors.New("invalid order quantity")
)
