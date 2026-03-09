package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/predictx/order-service/internal/domain"
)

// OrderRepo handles database operations for orders and RG limits.
type OrderRepo struct {
	db *pgxpool.Pool
}

// NewOrderRepo creates a new order repository.
func NewOrderRepo(db *pgxpool.Pool) *OrderRepo {
	return &OrderRepo{db: db}
}

// CreateOrder inserts a new order (idempotent via idempotency key).
func (r *OrderRepo) CreateOrder(ctx context.Context, order *domain.Order) (*domain.Order, error) {
	if order.ID == uuid.Nil {
		order.ID = uuid.New()
	}

	const q = `
		INSERT INTO orders (id, user_id, market_id, order_type, status, time_in_force,
			price_minor, quantity_shares, currency, outcome_index, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id, user_id, market_id, order_type, status, time_in_force,
			price_minor, quantity_shares, currency, outcome_index, idempotency_key, created_at, updated_at
	`

	now := order.CreatedAt
	if now.IsZero() {
		now = order.UpdatedAt
	}

	row := r.db.QueryRow(ctx, q,
		order.ID, order.UserID, order.MarketID, string(order.OrderType), string(order.Status),
		string(order.TimeInForce), order.PriceMinor, order.QuantityShares, order.Currency,
		order.OutcomeIndex, order.IdempotencyKey, now, now,
	)

	// If idempotency key already exists, fetch the existing order
	createdOrder := &domain.Order{}
	err := r.scanOrder(row, createdOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		// Idempotency hit — fetch existing order
		return r.GetOrderByIdempotencyKey(ctx, order.IdempotencyKey)
	}
	if err != nil {
		return nil, fmt.Errorf("order_repo: create_order: %w", err)
	}

	return createdOrder, nil
}

// GetOrder fetches an order by ID.
func (r *OrderRepo) GetOrder(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	const q = `
		SELECT id, user_id, market_id, order_type, status, time_in_force,
			price_minor, quantity_shares, currency, outcome_index, idempotency_key, created_at, updated_at
		FROM orders WHERE id = $1
	`

	row := r.db.QueryRow(ctx, q, orderID)
	order := &domain.Order{}
	if err := r.scanOrder(row, order); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, fmt.Errorf("order_repo: get_order: %w", err)
	}

	return order, nil
}

// GetOrderByIdempotencyKey fetches an order by idempotency key.
func (r *OrderRepo) GetOrderByIdempotencyKey(ctx context.Context, idempotencyKey string) (*domain.Order, error) {
	const q = `
		SELECT id, user_id, market_id, order_type, status, time_in_force,
			price_minor, quantity_shares, currency, outcome_index, idempotency_key, created_at, updated_at
		FROM orders WHERE idempotency_key = $1
	`

	row := r.db.QueryRow(ctx, q, idempotencyKey)
	order := &domain.Order{}
	if err := r.scanOrder(row, order); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, fmt.Errorf("order_repo: get_order_by_idempotency_key: %w", err)
	}

	return order, nil
}

// ListUserOrders fetches orders for a user with optional status filter.
func (r *OrderRepo) ListUserOrders(ctx context.Context, userID uuid.UUID, statusFilter domain.OrderStatus, limit, offset int) ([]*domain.Order, error) {
	var args []interface{}
	var conds []string
	argIndex := 1

	conds = append(conds, fmt.Sprintf("user_id = $%d", argIndex))
	args = append(args, userID)
	argIndex++

	if statusFilter != "" {
		conds = append(conds, fmt.Sprintf("status = $%d::order_status", argIndex))
		args = append(args, string(statusFilter))
		argIndex++
	}

	where := "WHERE " + strings.Join(conds, " AND ")

	q := fmt.Sprintf(`
		SELECT id, user_id, market_id, order_type, status, time_in_force,
			price_minor, quantity_shares, currency, outcome_index, idempotency_key, created_at, updated_at
		FROM orders %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("order_repo: list_user_orders: %w", err)
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		order := &domain.Order{}
		if err := r.scanOrder(rows, order); err != nil {
			return nil, fmt.Errorf("order_repo: scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("order_repo: list_user_orders rows error: %w", err)
	}

	return orders, nil
}

// ListPendingOrders fetches all pending orders for a market (for auto-cancel on market void).
func (r *OrderRepo) ListPendingOrders(ctx context.Context, marketID uuid.UUID) ([]*domain.Order, error) {
	const q = `
		SELECT id, user_id, market_id, order_type, status, time_in_force,
			price_minor, quantity_shares, currency, outcome_index, idempotency_key, created_at, updated_at
		FROM orders
		WHERE market_id = $1 AND status = 'pending'
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(ctx, q, marketID)
	if err != nil {
		return nil, fmt.Errorf("order_repo: list_pending_orders: %w", err)
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		order := &domain.Order{}
		if err := r.scanOrder(rows, order); err != nil {
			return nil, fmt.Errorf("order_repo: scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("order_repo: list_pending_orders rows error: %w", err)
	}

	return orders, nil
}

// UpdateStatus updates order status using optimistic concurrency (WHERE status = oldStatus).
// Returns ErrInvalidTransition if status is already changed (RowsAffected == 0).
func (r *OrderRepo) UpdateStatus(ctx context.Context, orderID uuid.UUID, newStatus, oldStatus domain.OrderStatus) error {
	const q = `
		UPDATE orders SET status = $1::order_status, updated_at = now()
		WHERE id = $2 AND status = $3::order_status
	`

	tag, err := r.db.Exec(ctx, q, string(newStatus), orderID, string(oldStatus))
	if err != nil {
		return fmt.Errorf("order_repo: update_status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidTransition
	}

	return nil
}

// GetRGLimit fetches responsible gambling limits for a user.
func (r *OrderRepo) GetRGLimit(ctx context.Context, userID uuid.UUID) (*domain.RGLimit, error) {
	const q = `
		SELECT id, user_id, daily_spent_minor, weekly_spent_minor, daily_reset_at, weekly_reset_at, updated_at
		FROM rg_limits WHERE user_id = $1
	`

	row := r.db.QueryRow(ctx, q, userID)
	limit := &domain.RGLimit{}

	err := row.Scan(&limit.ID, &limit.UserID, &limit.DailySpentMinor, &limit.WeeklySpentMinor,
		&limit.DailyResetAt, &limit.WeeklyResetAt, &limit.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // Not found is okay — create record on demand
	}
	if err != nil {
		return nil, fmt.Errorf("order_repo: get_rg_limit: %w", err)
	}

	return limit, nil
}

// CreateRGLimit inserts a new RG limit record (idempotent).
func (r *OrderRepo) CreateRGLimit(ctx context.Context, limit *domain.RGLimit) (*domain.RGLimit, error) {
	if limit.ID == uuid.Nil {
		limit.ID = uuid.New()
	}

	const q = `
		INSERT INTO rg_limits (id, user_id, daily_spent_minor, weekly_spent_minor, daily_reset_at, weekly_reset_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING id, user_id, daily_spent_minor, weekly_spent_minor, daily_reset_at, weekly_reset_at, updated_at
	`

	now := limit.UpdatedAt
	if now.IsZero() {
		return nil, fmt.Errorf("order_repo: rg_limit updated_at required")
	}

	row := r.db.QueryRow(ctx, q,
		limit.ID, limit.UserID, limit.DailySpentMinor, limit.WeeklySpentMinor,
		limit.DailyResetAt, limit.WeeklyResetAt, now,
	)

	created := &domain.RGLimit{}
	err := row.Scan(&created.ID, &created.UserID, &created.DailySpentMinor, &created.WeeklySpentMinor,
		&created.DailyResetAt, &created.WeeklyResetAt, &created.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Idempotency hit — fetch existing record
		return r.GetRGLimit(ctx, limit.UserID)
	}
	if err != nil {
		return nil, fmt.Errorf("order_repo: create_rg_limit: %w", err)
	}

	return created, nil
}

// UpdateRGLimit updates daily/weekly spend and reset times.
func (r *OrderRepo) UpdateRGLimit(ctx context.Context, limit *domain.RGLimit) error {
	const q = `
		UPDATE rg_limits SET daily_spent_minor = $1, weekly_spent_minor = $2,
			daily_reset_at = $3, weekly_reset_at = $4, updated_at = now()
		WHERE user_id = $5
	`

	_, err := r.db.Exec(ctx, q,
		limit.DailySpentMinor, limit.WeeklySpentMinor,
		limit.DailyResetAt, limit.WeeklyResetAt,
		limit.UserID,
	)
	if err != nil {
		return fmt.Errorf("order_repo: update_rg_limit: %w", err)
	}

	return nil
}

// scanner interface for scanning rows into domain types.
type scanner interface {
	Scan(dest ...interface{}) error
}

// scanOrder scans a single row into an Order domain object.
func (r *OrderRepo) scanOrder(row scanner, order *domain.Order) error {
	var orderTypeStr, statusStr, tifStr string

	err := row.Scan(
		&order.ID, &order.UserID, &order.MarketID,
		&orderTypeStr, &statusStr, &tifStr,
		&order.PriceMinor, &order.QuantityShares, &order.Currency,
		&order.OutcomeIndex, &order.IdempotencyKey,
		&order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		return err
	}

	order.OrderType = domain.OrderType(orderTypeStr)
	order.Status = domain.OrderStatus(statusStr)
	order.TimeInForce = domain.TimeInForce(tifStr)

	return nil
}
