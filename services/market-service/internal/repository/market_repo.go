package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/predictx/market-service/internal/domain"
)

// MarketRepo handles all market-related database operations.
type MarketRepo struct {
	db *pgxpool.Pool
}

func NewMarketRepo(db *pgxpool.Pool) *MarketRepo {
	return &MarketRepo{db: db}
}

// CreateMarket inserts a new market record.
func (r *MarketRepo) CreateMarket(ctx context.Context, m *domain.Market) (*domain.Market, error) {
	meta, err := json.Marshal(m.Metadata)
	if err != nil {
		meta = []byte("{}")
	}

	const q = `
		INSERT INTO markets
			(id, title, question, resolution_criteria, category, status,
			 creator_id, creator_type, pool_amount_minor, currency,
			 closes_at, resolves_at, metadata)
		VALUES ($1,$2,$3,$4,$5::market_category,$6::market_status,
		        $7,$8::creator_type,$9,$10,$11,$12,$13)
		RETURNING id, title, question, resolution_criteria, category, status,
		          creator_id, creator_type, pool_amount_minor, currency,
		          closes_at, resolves_at, metadata, created_at, updated_at`

	row := r.db.QueryRow(ctx, q,
		m.ID, m.Title, m.Question, m.ResolutionCriteria,
		string(m.Category), string(m.Status),
		m.CreatorID, string(m.CreatorType),
		m.PoolAmountMinor, m.Currency,
		m.ClosesAt, m.ResolvesAt, meta,
	)
	return r.scanMarket(row)
}

// GetMarket fetches a single market by ID.
func (r *MarketRepo) GetMarket(ctx context.Context, marketID uuid.UUID) (*domain.Market, error) {
	const q = `
		SELECT id, title, question, resolution_criteria, category, status,
		       creator_id, creator_type, pool_amount_minor, currency,
		       closes_at, resolves_at, metadata, created_at, updated_at
		FROM markets WHERE id = $1`

	m, err := r.scanMarket(r.db.QueryRow(ctx, q, marketID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrMarketNotFound
	}
	return m, err
}

// ListMarkets returns markets filtered by status and/or category with pagination.
// Zero-value filters are ignored.
func (r *MarketRepo) ListMarkets(ctx context.Context, f domain.ListFilters) ([]*domain.Market, error) {
	var conds []string
	var args []any
	i := 1

	if f.Status != "" {
		conds = append(conds, fmt.Sprintf("status = $%d::market_status", i))
		args = append(args, string(f.Status))
		i++
	}
	if f.Category != "" {
		conds = append(conds, fmt.Sprintf("category = $%d::market_category", i))
		args = append(args, string(f.Category))
		i++
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	q := fmt.Sprintf(`
		SELECT id, title, question, resolution_criteria, category, status,
		       creator_id, creator_type, pool_amount_minor, currency,
		       closes_at, resolves_at, metadata, created_at, updated_at
		FROM markets %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, i, i+1)

	args = append(args, limit, f.Offset)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("market_repo: list: %w", err)
	}
	defer rows.Close()

	var markets []*domain.Market
	for rows.Next() {
		m, err := r.scanMarket(rows)
		if err != nil {
			return nil, err
		}
		markets = append(markets, m)
	}
	return markets, rows.Err()
}

// ListResolvable returns active markets whose closes_at has passed.
// This is the endpoint used by the Resolution Service.
func (r *MarketRepo) ListResolvable(ctx context.Context) ([]*domain.Market, error) {
	const q = `
		SELECT id, title, question, resolution_criteria, category, status,
		       creator_id, creator_type, pool_amount_minor, currency,
		       closes_at, resolves_at, metadata, created_at, updated_at
		FROM markets
		WHERE status = 'active' AND closes_at <= now()
		ORDER BY closes_at ASC`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("market_repo: list_resolvable: %w", err)
	}
	defer rows.Close()

	var markets []*domain.Market
	for rows.Next() {
		m, err := r.scanMarket(rows)
		if err != nil {
			return nil, err
		}
		markets = append(markets, m)
	}
	return markets, rows.Err()
}

// UpdateStatus transitions a market to a new status.
// Uses optimistic concurrency: only updates if current status matches oldStatus.
// Returns ErrInvalidTransition if the row was already changed.
func (r *MarketRepo) UpdateStatus(ctx context.Context, marketID uuid.UUID, newStatus, oldStatus domain.MarketStatus) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE markets SET status = $1::market_status
		WHERE id = $2 AND status = $3::market_status`,
		string(newStatus), marketID, string(oldStatus),
	)
	if err != nil {
		return fmt.Errorf("market_repo: update_status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidTransition
	}
	return nil
}

// IncrementPool adds amountMinor to the market pool (called by Matching Engine / Settlement).
func (r *MarketRepo) IncrementPool(ctx context.Context, marketID uuid.UUID, amountMinor int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE markets SET pool_amount_minor = pool_amount_minor + $1 WHERE id = $2`,
		amountMinor, marketID,
	)
	return err
}

// ─── helpers ──────────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func (r *MarketRepo) scanMarket(row scanner) (*domain.Market, error) {
	m := &domain.Market{}
	var (
		cat, stat, ctype string
		metaRaw          []byte
	)
	err := row.Scan(
		&m.ID, &m.Title, &m.Question, &m.ResolutionCriteria,
		&cat, &stat,
		&m.CreatorID, &ctype,
		&m.PoolAmountMinor, &m.Currency,
		&m.ClosesAt, &m.ResolvesAt, &metaRaw,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	m.Category = domain.MarketCategory(cat)
	m.Status = domain.MarketStatus(stat)
	m.CreatorType = domain.CreatorType(ctype)
	_ = json.Unmarshal(metaRaw, &m.Metadata)
	return m, nil
}
