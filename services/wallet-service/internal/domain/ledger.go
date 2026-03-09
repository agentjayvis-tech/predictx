package domain

import (
	"time"

	"github.com/google/uuid"
)

// Transaction is the top-level record of an economic event.
// One transaction maps to exactly one ledger entry (single-sided accounting
// from the user's perspective; the system ledger is implicit).
type Transaction struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	IdempotencyKey string
	TxnType        TxnType
	Status         string
	Currency       Currency
	AmountMinor    int64
	Description    string
	ReferenceID    *uuid.UUID
	ReferenceType  string
	Metadata       map[string]any
	CreatedAt      time.Time
	CompletedAt    *time.Time
}

// LedgerEntry is an immutable audit record of a single balance change.
type LedgerEntry struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	WalletID           uuid.UUID
	TransactionID      uuid.UUID
	EntryType          EntryType
	AmountMinor        int64
	BalanceAfterMinor  int64
	Currency           Currency
	Description        string
	CreatedAt          time.Time
}

// ApplyTxnRequest is the input to the repository's ApplyTransaction method.
type ApplyTxnRequest struct {
	UserID         uuid.UUID
	IdempotencyKey string
	TxnType        TxnType
	Currency       Currency
	AmountMinor    int64
	EntryType      EntryType // credit or debit
	Description    string
	ReferenceID    *uuid.UUID
	ReferenceType  string
	Metadata       map[string]any
}
