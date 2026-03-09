package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/predictx/settlement-service/internal/domain"
)

// SettlementRepo handles all settlement and position database operations.
// All queries use pgx async pool (non-blocking connection acquisition).
type SettlementRepo struct {
	db *pgxpool.Pool
}

func NewSettlementRepo(db *pgxpool.Pool) *SettlementRepo {
	return &SettlementRepo{db: db}
}

// ─── Position operations ──────────────────────────────────────────────────────

// UpsertPosition inserts a new position or increments stake on an existing one.
// Atomic via INSERT ... ON CONFLICT DO UPDATE.
func (r *SettlementRepo) UpsertPosition(ctx context.Context, p *domain.Position) (*domain.Position, error) {
	const q = `
		INSERT INTO positions
			(id, user_id, market_id, outcome_index, stake_minor, currency, status, order_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7::position_status, 1)
		ON CONFLICT (user_id, market_id, outcome_index)
		DO UPDATE SET
			stake_minor = positions.stake_minor + EXCLUDED.stake_minor,
			order_count = positions.order_count + 1,
			updated_at  = now()
		RETURNING id, user_id, market_id, outcome_index, stake_minor, currency,
		          status, order_count, created_at, updated_at`

	row := r.db.QueryRow(ctx, q,
		p.ID, p.UserID, p.MarketID, p.OutcomeIndex,
		p.StakeMinor, p.Currency, string(p.Status),
	)
	return r.scanPosition(row)
}

// GetPosition fetches a single position by user/market/outcome (the natural key).
func (r *SettlementRepo) GetPosition(ctx context.Context, userID, marketID uuid.UUID, outcomeIndex int) (*domain.Position, error) {
	const q = `
		SELECT id, user_id, market_id, outcome_index, stake_minor, currency,
		       status, order_count, created_at, updated_at
		FROM positions
		WHERE user_id = $1 AND market_id = $2 AND outcome_index = $3`

	pos, err := r.scanPosition(r.db.QueryRow(ctx, q, userID, marketID, outcomeIndex))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrPositionNotFound
	}
	return pos, err
}

// ListPositionsByMarket returns all positions for a given market.
func (r *SettlementRepo) ListPositionsByMarket(ctx context.Context, marketID uuid.UUID) ([]*domain.Position, error) {
	const q = `
		SELECT id, user_id, market_id, outcome_index, stake_minor, currency,
		       status, order_count, created_at, updated_at
		FROM positions
		WHERE market_id = $1
		ORDER BY created_at ASC`

	rows, err := r.db.Query(ctx, q, marketID)
	if err != nil {
		return nil, fmt.Errorf("repo: list_positions: %w", err)
	}
	defer rows.Close()

	var positions []*domain.Position
	for rows.Next() {
		pos, err := r.scanPosition(rows)
		if err != nil {
			return nil, err
		}
		positions = append(positions, pos)
	}
	return positions, rows.Err()
}

// ListUserPositions returns all open positions for a user.
func (r *SettlementRepo) ListUserPositions(ctx context.Context, userID uuid.UUID) ([]*domain.Position, error) {
	const q = `
		SELECT id, user_id, market_id, outcome_index, stake_minor, currency,
		       status, order_count, created_at, updated_at
		FROM positions
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("repo: list_user_positions: %w", err)
	}
	defer rows.Close()

	var positions []*domain.Position
	for rows.Next() {
		pos, err := r.scanPosition(rows)
		if err != nil {
			return nil, err
		}
		positions = append(positions, pos)
	}
	return positions, rows.Err()
}

// UpdatePositionStatus transitions positions for a market to a new status.
// Uses optimistic concurrency via current status constraint.
func (r *SettlementRepo) UpdatePositionStatusByMarket(ctx context.Context, marketID uuid.UUID, newStatus, expectedStatus domain.PositionStatus) error {
	_, err := r.db.Exec(ctx, `
		UPDATE positions
		SET status = $1::position_status, updated_at = now()
		WHERE market_id = $2 AND status = $3::position_status`,
		string(newStatus), marketID, string(expectedStatus),
	)
	return err
}

// ─── Settlement operations ────────────────────────────────────────────────────

// GetSettlementByMarket fetches a settlement for a market, or ErrSettlementNotFound.
func (r *SettlementRepo) GetSettlementByMarket(ctx context.Context, marketID uuid.UUID) (*domain.Settlement, error) {
	const q = `
		SELECT id, market_id, resolution_id, status, winning_outcome,
		       total_pool_minor, insurance_fee_minor, net_pool_minor,
		       winning_stake_minor, winner_count, loser_count, currency,
		       settled_at, created_at, updated_at
		FROM settlements
		WHERE market_id = $1`

	s, err := r.scanSettlement(r.db.QueryRow(ctx, q, marketID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrSettlementNotFound
	}
	return s, err
}

// CreateSettlement inserts a new settlement record (must not exist).
func (r *SettlementRepo) CreateSettlement(ctx context.Context, s *domain.Settlement) (*domain.Settlement, error) {
	const q = `
		INSERT INTO settlements
			(id, market_id, resolution_id, status, winning_outcome,
			 total_pool_minor, insurance_fee_minor, net_pool_minor,
			 winning_stake_minor, winner_count, loser_count, currency)
		VALUES ($1,$2,$3,$4::settlement_status,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, market_id, resolution_id, status, winning_outcome,
		          total_pool_minor, insurance_fee_minor, net_pool_minor,
		          winning_stake_minor, winner_count, loser_count, currency,
		          settled_at, created_at, updated_at`

	row := r.db.QueryRow(ctx, q,
		s.ID, s.MarketID, s.ResolutionID, string(s.Status),
		s.WinningOutcome, s.TotalPoolMinor, s.InsuranceFeeMinor,
		s.NetPoolMinor, s.WinningStakeMinor, s.WinnerCount, s.LoserCount,
		s.Currency,
	)
	return r.scanSettlement(row)
}

// UpdateSettlementStatus transitions the settlement to a new status.
func (r *SettlementRepo) UpdateSettlementStatus(ctx context.Context, settlementID uuid.UUID, newStatus domain.SettlementStatus) error {
	q := `UPDATE settlements SET status = $1::settlement_status, updated_at = now() WHERE id = $2`
	if newStatus == domain.SettlementStatusCompleted {
		q = `UPDATE settlements SET status = $1::settlement_status, settled_at = now(), updated_at = now() WHERE id = $2`
	}
	_, err := r.db.Exec(ctx, q, string(newStatus), settlementID)
	return err
}

// ─── Settlement entry operations ──────────────────────────────────────────────

// CreateSettlementEntries bulk-inserts all entries for a settlement.
func (r *SettlementRepo) CreateSettlementEntries(ctx context.Context, entries []*domain.SettlementEntry) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repo: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, e := range entries {
		_, err := tx.Exec(ctx, `
			INSERT INTO settlement_entries
				(id, settlement_id, user_id, position_id,
				 stake_minor, payout_minor, pnl_minor,
				 is_winner, status, idempotency_key)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::entry_status,$10)
			ON CONFLICT (idempotency_key) DO NOTHING`,
			e.ID, e.SettlementID, e.UserID, e.PositionID,
			e.StakeMinor, e.PayoutMinor, e.PnlMinor,
			e.IsWinner, string(e.Status), e.IdempotencyKey,
		)
		if err != nil {
			return fmt.Errorf("repo: insert entry: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// UpdateEntryStatus updates the status of a single settlement entry.
func (r *SettlementRepo) UpdateEntryStatus(ctx context.Context, entryID uuid.UUID, status domain.EntryStatus) error {
	_, err := r.db.Exec(ctx, `
		UPDATE settlement_entries
		SET status = $1::entry_status, updated_at = now()
		WHERE id = $2`,
		string(status), entryID,
	)
	return err
}

// ListPendingEntries returns all pending payout entries for a settlement.
func (r *SettlementRepo) ListPendingEntries(ctx context.Context, settlementID uuid.UUID) ([]*domain.SettlementEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, settlement_id, user_id, position_id,
		       stake_minor, payout_minor, pnl_minor,
		       is_winner, status, idempotency_key, created_at, updated_at
		FROM settlement_entries
		WHERE settlement_id = $1 AND status = 'pending'
		ORDER BY created_at ASC`,
		settlementID,
	)
	if err != nil {
		return nil, fmt.Errorf("repo: list_pending_entries: %w", err)
	}
	defer rows.Close()

	var entries []*domain.SettlementEntry
	for rows.Next() {
		e, err := r.scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// InsertFraudAlert records a suspected fraud event.
func (r *SettlementRepo) InsertFraudAlert(ctx context.Context, alert *domain.FraudAlert) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO fraud_alerts (id, market_id, outcome_index, reason, severity, detected_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		alert.ID, alert.MarketID, alert.OutcomeIndex,
		alert.Reason, alert.Severity, alert.DetectedAt,
	)
	return err
}

// ─── helpers ──────────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func (r *SettlementRepo) scanPosition(row scanner) (*domain.Position, error) {
	p := &domain.Position{}
	var status string
	err := row.Scan(
		&p.ID, &p.UserID, &p.MarketID, &p.OutcomeIndex,
		&p.StakeMinor, &p.Currency, &status, &p.OrderCount,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.Status = domain.PositionStatus(status)
	return p, nil
}

func (r *SettlementRepo) scanSettlement(row scanner) (*domain.Settlement, error) {
	s := &domain.Settlement{}
	var status string
	err := row.Scan(
		&s.ID, &s.MarketID, &s.ResolutionID, &status, &s.WinningOutcome,
		&s.TotalPoolMinor, &s.InsuranceFeeMinor, &s.NetPoolMinor,
		&s.WinningStakeMinor, &s.WinnerCount, &s.LoserCount, &s.Currency,
		&s.SettledAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.Status = domain.SettlementStatus(status)
	return s, nil
}

func (r *SettlementRepo) scanEntry(row scanner) (*domain.SettlementEntry, error) {
	e := &domain.SettlementEntry{}
	var status string
	err := row.Scan(
		&e.ID, &e.SettlementID, &e.UserID, &e.PositionID,
		&e.StakeMinor, &e.PayoutMinor, &e.PnlMinor,
		&e.IsWinner, &status, &e.IdempotencyKey,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	e.Status = domain.EntryStatus(status)
	return e, nil
}
