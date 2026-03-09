package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/predictx/wallet-service/internal/domain"
)

// WalletRepo handles all wallet-related database operations.
type WalletRepo struct {
	db *pgxpool.Pool
}

func NewWalletRepo(db *pgxpool.Pool) *WalletRepo {
	return &WalletRepo{db: db}
}

// GetOrCreateWallet returns an existing wallet or creates one with zero balance.
func (r *WalletRepo) GetOrCreateWallet(ctx context.Context, userID uuid.UUID, currency domain.Currency) (*domain.Wallet, error) {
	const q = `
		INSERT INTO wallets (user_id, currency)
		VALUES ($1, $2)
		ON CONFLICT (user_id, currency) DO NOTHING
		RETURNING id, user_id, currency, balance_minor, is_active, created_at, updated_at`

	w, err := r.scanWallet(r.db.QueryRow(ctx, q, userID, string(currency)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Row already existed — fetch it.
			return r.GetWallet(ctx, userID, currency)
		}
		return nil, fmt.Errorf("wallet_repo: get_or_create: %w", err)
	}
	return w, nil
}

// GetWallet fetches a wallet by user_id + currency.
func (r *WalletRepo) GetWallet(ctx context.Context, userID uuid.UUID, currency domain.Currency) (*domain.Wallet, error) {
	const q = `
		SELECT id, user_id, currency, balance_minor, is_active, created_at, updated_at
		FROM wallets
		WHERE user_id = $1 AND currency = $2`

	w, err := r.scanWallet(r.db.QueryRow(ctx, q, userID, string(currency)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrWalletNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: get_wallet: %w", err)
	}
	return w, nil
}

// GetAllWallets returns all wallets for a user.
func (r *WalletRepo) GetAllWallets(ctx context.Context, userID uuid.UUID) ([]*domain.Wallet, error) {
	const q = `
		SELECT id, user_id, currency, balance_minor, is_active, created_at, updated_at
		FROM wallets
		WHERE user_id = $1
		ORDER BY currency`

	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: get_all_wallets: %w", err)
	}
	defer rows.Close()

	var wallets []*domain.Wallet
	for rows.Next() {
		w, err := r.scanWallet(rows)
		if err != nil {
			return nil, err
		}
		wallets = append(wallets, w)
	}
	return wallets, rows.Err()
}

// ApplyTransaction executes an atomic double-entry transaction:
//  1. Inserts the transaction record (with idempotency key).
//  2. Calls apply_double_entry() stored proc to update balance + ledger.
//
// On duplicate idempotency_key, returns the existing transaction without error.
func (r *WalletRepo) ApplyTransaction(ctx context.Context, req domain.ApplyTxnRequest) (*domain.Transaction, error) {
	if req.AmountMinor <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	wallet, err := r.GetWallet(ctx, req.UserID, req.Currency)
	if errors.Is(err, domain.ErrWalletNotFound) {
		return nil, domain.ErrWalletNotFound
	}
	if err != nil {
		return nil, err
	}
	if !wallet.IsActive {
		return nil, domain.ErrWalletFrozen
	}

	metaJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	// Begin transaction
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Insert transaction row (idempotent via ON CONFLICT DO NOTHING).
	const insertTxn = `
		INSERT INTO transactions
			(user_id, idempotency_key, txn_type, currency, amount_minor,
			 description, reference_id, reference_type, metadata, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id, created_at`

	var txnID uuid.UUID
	var createdAt time.Time
	err = tx.QueryRow(ctx, insertTxn,
		req.UserID, req.IdempotencyKey, string(req.TxnType), string(req.Currency),
		req.AmountMinor, req.Description, req.ReferenceID, req.ReferenceType, metaJSON,
	).Scan(&txnID, &createdAt)

	if errors.Is(err, pgx.ErrNoRows) {
		// Idempotency hit — fetch and return existing transaction.
		tx.Rollback(ctx) //nolint:errcheck
		return r.getTransactionByIdempotencyKey(ctx, req.IdempotencyKey)
	}
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: insert txn: %w", err)
	}

	// Call stored proc to atomically update balance + insert ledger entry.
	var newBalance int64
	err = tx.QueryRow(ctx, `SELECT apply_double_entry($1, $2, $3, $4::entry_type, $5, $6)`,
		req.UserID, wallet.ID, txnID, string(req.EntryType), req.AmountMinor, req.Description,
	).Scan(&newBalance)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Message == "insufficient_funds" {
				return nil, domain.ErrInsufficientFunds
			}
			if pgErr.Message == "wallet_not_found" {
				return nil, domain.ErrWalletNotFound
			}
		}
		return nil, fmt.Errorf("wallet_repo: apply_double_entry: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("wallet_repo: commit: %w", err)
	}

	now := time.Now()
	return &domain.Transaction{
		ID:             txnID,
		UserID:         req.UserID,
		IdempotencyKey: req.IdempotencyKey,
		TxnType:        req.TxnType,
		Status:         "completed",
		Currency:       req.Currency,
		AmountMinor:    req.AmountMinor,
		Description:    req.Description,
		ReferenceID:    req.ReferenceID,
		ReferenceType:  req.ReferenceType,
		Metadata:       req.Metadata,
		CreatedAt:      createdAt,
		CompletedAt:    &now,
	}, nil
}

// ListTransactions returns paginated transaction history for a user+currency.
func (r *WalletRepo) ListTransactions(ctx context.Context, userID uuid.UUID, currency domain.Currency, limit, offset int) ([]*domain.Transaction, error) {
	const q = `
		SELECT id, user_id, idempotency_key, txn_type, status, currency,
		       amount_minor, description, reference_id, reference_type,
		       metadata, created_at, completed_at
		FROM transactions
		WHERE user_id = $1 AND currency = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(ctx, q, userID, string(currency), limit, offset)
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: list_txns: %w", err)
	}
	defer rows.Close()

	var txns []*domain.Transaction
	for rows.Next() {
		t := &domain.Transaction{}
		var metaRaw []byte
		err := rows.Scan(
			&t.ID, &t.UserID, &t.IdempotencyKey, &t.TxnType, &t.Status, &t.Currency,
			&t.AmountMinor, &t.Description, &t.ReferenceID, &t.ReferenceType,
			&metaRaw, &t.CreatedAt, &t.CompletedAt,
		)
		if err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metaRaw, &t.Metadata)
		txns = append(txns, t)
	}
	return txns, rows.Err()
}

// InsertFraudAlert persists a fraud detection alert.
func (r *WalletRepo) InsertFraudAlert(ctx context.Context, alert *domain.FraudAlert) error {
	details, _ := json.Marshal(alert.Details)
	_, err := r.db.Exec(ctx, `
		INSERT INTO fraud_alerts (id, user_id, wallet_id, alert_type, details)
		VALUES ($1, $2, $3, $4, $5)`,
		alert.ID, alert.UserID, alert.WalletID, string(alert.AlertType), details,
	)
	return err
}

// CountRecentLedgerEntries returns how many ledger entries exist for a wallet in the last windowSecs.
func (r *WalletRepo) CountRecentLedgerEntries(ctx context.Context, userID, walletID uuid.UUID, windowSecs int) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ledger_entries
		WHERE user_id = $1 AND wallet_id = $2
		  AND created_at > now() - ($3 || ' seconds')::interval`,
		userID, walletID, windowSecs,
	).Scan(&count)
	return count, err
}

// GetBalanceAtTime returns the last known balance_after_minor before a given time.
func (r *WalletRepo) GetBalanceAtTime(ctx context.Context, userID, walletID uuid.UUID, at time.Time) (int64, error) {
	var balance int64
	err := r.db.QueryRow(ctx, `
		SELECT balance_after_minor FROM ledger_entries
		WHERE user_id = $1 AND wallet_id = $2 AND created_at <= $3
		ORDER BY created_at DESC LIMIT 1`,
		userID, walletID, at,
	).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return balance, err
}

// ─── Responsible Gambling Methods ─────────────────────────────────────────────

// GetDepositSettings returns user's deposit limit settings or creates defaults.
func (r *WalletRepo) GetDepositSettings(ctx context.Context, userID uuid.UUID) (*domain.DepositSettings, error) {
	const q = `
		INSERT INTO user_deposit_settings (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING user_id, daily_deposit_limit_minor, monthly_deposit_limit_minor, enabled, created_at, updated_at`

	ds := &domain.DepositSettings{}
	err := r.db.QueryRow(ctx, q, userID).Scan(
		&ds.UserID, &ds.DailyDepositLimitMinor, &ds.MonthlyDepositLimitMinor, &ds.Enabled, &ds.CreatedAt, &ds.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		// Row already existed — fetch it
		return r.getDepositSettingsExisting(ctx, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: get_deposit_settings: %w", err)
	}
	return ds, nil
}

// getDepositSettingsExisting fetches an existing deposit settings row.
func (r *WalletRepo) getDepositSettingsExisting(ctx context.Context, userID uuid.UUID) (*domain.DepositSettings, error) {
	const q = `
		SELECT user_id, daily_deposit_limit_minor, monthly_deposit_limit_minor, enabled, created_at, updated_at
		FROM user_deposit_settings
		WHERE user_id = $1`

	ds := &domain.DepositSettings{}
	err := r.db.QueryRow(ctx, q, userID).Scan(
		&ds.UserID, &ds.DailyDepositLimitMinor, &ds.MonthlyDepositLimitMinor, &ds.Enabled, &ds.CreatedAt, &ds.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: get_deposit_settings_existing: %w", err)
	}
	return ds, nil
}

// UpdateDepositSettings updates user's daily/monthly deposit limits.
func (r *WalletRepo) UpdateDepositSettings(ctx context.Context, userID uuid.UUID, dailyMinor int64, monthlyMinor *int64) (*domain.DepositSettings, error) {
	const q = `
		UPDATE user_deposit_settings
		SET daily_deposit_limit_minor = $1, monthly_deposit_limit_minor = $2
		WHERE user_id = $3
		RETURNING user_id, daily_deposit_limit_minor, monthly_deposit_limit_minor, enabled, created_at, updated_at`

	ds := &domain.DepositSettings{}
	err := r.db.QueryRow(ctx, q, dailyMinor, monthlyMinor, userID).Scan(
		&ds.UserID, &ds.DailyDepositLimitMinor, &ds.MonthlyDepositLimitMinor, &ds.Enabled, &ds.CreatedAt, &ds.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: update_deposit_settings: %w", err)
	}
	return ds, nil
}

// RecordDailyDeposit atomically increments the daily deposit total for today.
func (r *WalletRepo) RecordDailyDeposit(ctx context.Context, userID uuid.UUID, amountMinor int64) (*domain.DailyDepositTracking, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	const q = `
		INSERT INTO daily_deposit_tracking (user_id, tracked_date, total_deposited_minor)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, tracked_date) DO UPDATE
		SET total_deposited_minor = total_deposited_minor + $3
		RETURNING user_id, tracked_date, total_deposited_minor, created_at, updated_at`

	ddt := &domain.DailyDepositTracking{}
	err := r.db.QueryRow(ctx, q, userID, today, amountMinor).Scan(
		&ddt.UserID, &ddt.TrackedDate, &ddt.TotalDepositedMinor, &ddt.CreatedAt, &ddt.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: record_daily_deposit: %w", err)
	}
	return ddt, nil
}

// GetDailyDepositTotal returns the total deposits for a specific date.
func (r *WalletRepo) GetDailyDepositTotal(ctx context.Context, userID uuid.UUID, date time.Time) (int64, error) {
	normalizedDate := date.UTC().Truncate(24 * time.Hour)

	var total int64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(total_deposited_minor, 0)
		FROM daily_deposit_tracking
		WHERE user_id = $1 AND tracked_date = $2`,
		userID, normalizedDate,
	).Scan(&total)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("wallet_repo: get_daily_deposit_total: %w", err)
	}
	return total, nil
}

// GetExclusionSettings returns user's cool-off and self-exclusion settings or creates defaults.
func (r *WalletRepo) GetExclusionSettings(ctx context.Context, userID uuid.UUID) (*domain.ExclusionSettings, error) {
	const q = `
		INSERT INTO user_exclusion_settings (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING user_id, country_code, cool_off_until, cool_off_duration_hours,
		          cool_off_cancellable, is_self_excluded, self_excluded_at,
		          self_exclusion_duration_days, created_at, updated_at`

	es := &domain.ExclusionSettings{}
	err := r.db.QueryRow(ctx, q, userID).Scan(
		&es.UserID, &es.CountryCode, &es.CoolOffUntil, &es.CoolOffDurationHours,
		&es.CoolOffCancellable, &es.IsSelfExcluded, &es.SelfExcludedAt,
		&es.SelfExclusionDurationDays, &es.CreatedAt, &es.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		// Row already existed — fetch it
		return r.getExclusionSettingsExisting(ctx, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: get_exclusion_settings: %w", err)
	}
	return es, nil
}

// getExclusionSettingsExisting fetches an existing exclusion settings row.
func (r *WalletRepo) getExclusionSettingsExisting(ctx context.Context, userID uuid.UUID) (*domain.ExclusionSettings, error) {
	const q = `
		SELECT user_id, country_code, cool_off_until, cool_off_duration_hours,
		       cool_off_cancellable, is_self_excluded, self_excluded_at,
		       self_exclusion_duration_days, created_at, updated_at
		FROM user_exclusion_settings
		WHERE user_id = $1`

	es := &domain.ExclusionSettings{}
	err := r.db.QueryRow(ctx, q, userID).Scan(
		&es.UserID, &es.CountryCode, &es.CoolOffUntil, &es.CoolOffDurationHours,
		&es.CoolOffCancellable, &es.IsSelfExcluded, &es.SelfExcludedAt,
		&es.SelfExclusionDurationDays, &es.CreatedAt, &es.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: get_exclusion_settings_existing: %w", err)
	}
	return es, nil
}

// UpdateExclusionSettings updates cool-off and self-exclusion state.
func (r *WalletRepo) UpdateExclusionSettings(ctx context.Context, userID uuid.UUID, es *domain.ExclusionSettings) (*domain.ExclusionSettings, error) {
	const q = `
		UPDATE user_exclusion_settings
		SET country_code = $1, cool_off_until = $2, cool_off_duration_hours = $3,
		    cool_off_cancellable = $4, is_self_excluded = $5, self_excluded_at = $6,
		    self_exclusion_duration_days = $7
		WHERE user_id = $8
		RETURNING user_id, country_code, cool_off_until, cool_off_duration_hours,
		          cool_off_cancellable, is_self_excluded, self_excluded_at,
		          self_exclusion_duration_days, created_at, updated_at`

	updated := &domain.ExclusionSettings{}
	err := r.db.QueryRow(ctx, q,
		es.CountryCode, es.CoolOffUntil, es.CoolOffDurationHours,
		es.CoolOffCancellable, es.IsSelfExcluded, es.SelfExcludedAt,
		es.SelfExclusionDurationDays, userID,
	).Scan(
		&updated.UserID, &updated.CountryCode, &updated.CoolOffUntil, &updated.CoolOffDurationHours,
		&updated.CoolOffCancellable, &updated.IsSelfExcluded, &updated.SelfExcludedAt,
		&updated.SelfExclusionDurationDays, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: update_exclusion_settings: %w", err)
	}
	return updated, nil
}

// GetCountryRGPolicy returns the RG policy configuration for a specific country.
func (r *WalletRepo) GetCountryRGPolicy(ctx context.Context, countryCode string) (*domain.CountryRGPolicy, error) {
	const q = `
		SELECT country_code, cool_off_cancellable, max_daily_deposit_limit_minor,
		       max_cool_off_duration_hours, created_at, updated_at
		FROM country_rg_policy
		WHERE country_code = $1`

	policy := &domain.CountryRGPolicy{}
	err := r.db.QueryRow(ctx, q, countryCode).Scan(
		&policy.CountryCode, &policy.CoolOffCancellable, &policy.MaxDailyDepositLimitMinor,
		&policy.MaxCoolOffDurationHours, &policy.CreatedAt, &policy.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil  // Country not configured; use global defaults
	}
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: get_country_rg_policy: %w", err)
	}
	return policy, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func (r *WalletRepo) scanWallet(row scanner) (*domain.Wallet, error) {
	w := &domain.Wallet{}
	var curr string
	err := row.Scan(&w.ID, &w.UserID, &curr, &w.BalanceMinor, &w.IsActive, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	w.Currency = domain.Currency(curr)
	return w, nil
}

func (r *WalletRepo) getTransactionByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	t := &domain.Transaction{}
	var metaRaw []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, idempotency_key, txn_type, status, currency,
		       amount_minor, description, reference_id, reference_type,
		       metadata, created_at, completed_at
		FROM transactions WHERE idempotency_key = $1`, key,
	).Scan(
		&t.ID, &t.UserID, &t.IdempotencyKey, &t.TxnType, &t.Status, &t.Currency,
		&t.AmountMinor, &t.Description, &t.ReferenceID, &t.ReferenceType,
		&metaRaw, &t.CreatedAt, &t.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("wallet_repo: fetch_idempotent: %w", err)
	}
	_ = json.Unmarshal(metaRaw, &t.Metadata)
	return t, nil
}

