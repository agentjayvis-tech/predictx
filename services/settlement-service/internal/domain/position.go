package domain

import (
	"time"

	"github.com/google/uuid"
)

// PositionStatus tracks the lifecycle of a user's market position.
type PositionStatus string

const (
	PositionStatusOpen     PositionStatus = "open"
	PositionStatusSettled  PositionStatus = "settled"
	PositionStatusRefunded PositionStatus = "refunded"
	PositionStatusSettling PositionStatus = "settling" // lock during settlement phase
)

// Position aggregates a user's total stake on a specific outcome in a market.
// Multiple matched orders on the same (user, market, outcome) are aggregated here.
type Position struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	MarketID     uuid.UUID
	OutcomeIndex int    // 0=NO / 1=YES for binary; index for scalar
	StakeMinor   int64  // total accumulated stake in minor units
	Currency     string // e.g. "COINS"
	Status       PositionStatus
	OrderCount   int // number of matched orders aggregated
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SettlementStatus tracks the lifecycle of a market settlement.
type SettlementStatus string

const (
	SettlementStatusPending    SettlementStatus = "pending"
	SettlementStatusProcessing SettlementStatus = "processing"
	SettlementStatusCompleted  SettlementStatus = "completed"
	SettlementStatusFailed     SettlementStatus = "failed"
)

// Settlement records the final settlement of a market after resolution.
// There is exactly one Settlement per resolved market (idempotent).
type Settlement struct {
	ID                uuid.UUID
	MarketID          uuid.UUID
	ResolutionID      string // from Resolution Service event
	Status            SettlementStatus
	WinningOutcome    int   // resolved outcome index
	TotalPoolMinor    int64 // sum of all stakes
	InsuranceFeeMinor int64 // 50bps of TotalPoolMinor
	NetPoolMinor      int64 // TotalPoolMinor - InsuranceFeeMinor
	WinningStakeMinor int64 // sum of stakes on winning side
	WinnerCount       int
	LoserCount        int
	Currency          string
	SettledAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// EntryStatus tracks whether a payout entry has been paid.
type EntryStatus string

const (
	EntryStatusPending EntryStatus = "pending"
	EntryStatusPaid    EntryStatus = "paid"
	EntryStatusFailed  EntryStatus = "failed"
	EntryStatusSkipped EntryStatus = "skipped" // losers
)

// SettlementEntry records the payout (or skip) for a single position.
type SettlementEntry struct {
	ID             uuid.UUID
	SettlementID   uuid.UUID
	UserID         uuid.UUID
	PositionID     uuid.UUID
	StakeMinor     int64
	PayoutMinor    int64  // 0 for losers; proportional share of NetPool for winners
	PnlMinor       int64  // PayoutMinor - StakeMinor
	IsWinner       bool
	Status         EntryStatus
	IdempotencyKey string // wallet gRPC idempotency key
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// FraudAlert records a suspected coordinated price manipulation event.
type FraudAlert struct {
	ID           uuid.UUID
	MarketID     uuid.UUID
	OutcomeIndex int
	Reason       string
	Severity     string // "low" | "medium" | "high"
	DetectedAt   time.Time
}

// InsuranceFeeBps is 50 basis points (0.5% of pool).
const InsuranceFeeBps = 50

// ComputeInsuranceFee returns the insurance fee for the given pool amount.
func ComputeInsuranceFee(totalPoolMinor int64) int64 {
	return totalPoolMinor * InsuranceFeeBps / 10_000
}

// ComputePayout returns a winner's proportional share of the net pool.
// payout = (winnerStake / totalWinningStake) * netPool
func ComputePayout(winnerStakeMinor, totalWinningStakeMinor, netPoolMinor int64) int64 {
	if totalWinningStakeMinor == 0 {
		return 0
	}
	// Use integer math: multiply first to preserve precision.
	return winnerStakeMinor * netPoolMinor / totalWinningStakeMinor
}
