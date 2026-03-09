package domain

import "errors"

var (
	ErrMarketNotFound    = errors.New("market_not_found")
	ErrInvalidTransition = errors.New("invalid_status_transition")
	ErrInvalidCategory   = errors.New("invalid_category")
	ErrClosesAtInPast    = errors.New("closes_at_in_past")
)
