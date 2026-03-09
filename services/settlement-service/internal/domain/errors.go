package domain

import "errors"

var (
	ErrPositionNotFound    = errors.New("position not found")
	ErrSettlementNotFound  = errors.New("settlement not found")
	ErrAlreadySettled      = errors.New("market already settled")
	ErrSettlementFailed    = errors.New("settlement failed")
	ErrInvalidOutcome      = errors.New("invalid winning outcome")
	ErrNoPositions         = errors.New("no positions found for market")
	ErrPositionConflict    = errors.New("position already exists with different currency")
)
