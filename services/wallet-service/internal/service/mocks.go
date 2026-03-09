package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/predictx/wallet-service/internal/domain"
)

// MockWalletRepo is a test double for repository.WalletRepo
type MockWalletRepo struct {
	wallets map[string]*domain.Wallet
	txns    map[string]*domain.Transaction
}

func NewMockWalletRepo() *MockWalletRepo {
	return &MockWalletRepo{
		wallets: make(map[string]*domain.Wallet),
		txns:    make(map[string]*domain.Transaction),
	}
}

func (m *MockWalletRepo) GetOrCreateWallet(ctx context.Context, userID uuid.UUID, currency domain.Currency) (*domain.Wallet, error) {
	key := userID.String() + ":" + string(currency)
	if w, ok := m.wallets[key]; ok {
		return w, nil
	}
	w := &domain.Wallet{
		ID:        uuid.New(),
		UserID:    userID,
		Currency:  currency,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.wallets[key] = w
	return w, nil
}

func (m *MockWalletRepo) GetWallet(ctx context.Context, userID uuid.UUID, currency domain.Currency) (*domain.Wallet, error) {
	key := userID.String() + ":" + string(currency)
	if w, ok := m.wallets[key]; ok {
		return w, nil
	}
	return nil, domain.ErrWalletNotFound
}

func (m *MockWalletRepo) GetAllWallets(ctx context.Context, userID uuid.UUID) ([]*domain.Wallet, error) {
	var result []*domain.Wallet
	for _, w := range m.wallets {
		if w.UserID == userID {
			result = append(result, w)
		}
	}
	return result, nil
}

func (m *MockWalletRepo) ApplyTransaction(ctx context.Context, req domain.ApplyTxnRequest) (*domain.Transaction, error) {
	if req.AmountMinor <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	wallet, err := m.GetWallet(ctx, req.UserID, req.Currency)
	if err != nil {
		return nil, err
	}

	// Check idempotency
	key := req.IdempotencyKey
	if t, ok := m.txns[key]; ok {
		return t, nil // Already processed
	}

	// Simulate balance check
	if req.EntryType == domain.EntryTypeDebit && wallet.BalanceMinor < req.AmountMinor {
		return nil, domain.ErrInsufficientFunds
	}

	// Update balance
	if req.EntryType == domain.EntryTypeCredit {
		wallet.BalanceMinor += req.AmountMinor
	} else {
		wallet.BalanceMinor -= req.AmountMinor
	}

	txn := &domain.Transaction{
		ID:             uuid.New(),
		UserID:         req.UserID,
		IdempotencyKey: req.IdempotencyKey,
		TxnType:        req.TxnType,
		Status:         "completed",
		Currency:       req.Currency,
		AmountMinor:    req.AmountMinor,
		Description:    req.Description,
		CreatedAt:      time.Now(),
	}
	m.txns[key] = txn
	return txn, nil
}

func (m *MockWalletRepo) ListTransactions(ctx context.Context, userID uuid.UUID, currency domain.Currency, limit, offset int) ([]*domain.Transaction, error) {
	return nil, nil
}

func (m *MockWalletRepo) InsertFraudAlert(ctx context.Context, alert *domain.FraudAlert) error {
	return nil
}

func (m *MockWalletRepo) CountRecentLedgerEntries(ctx context.Context, userID, walletID uuid.UUID, windowSecs int) (int, error) {
	return 0, nil
}

func (m *MockWalletRepo) GetBalanceAtTime(ctx context.Context, userID, walletID uuid.UUID, at time.Time) (int64, error) {
	return 0, nil
}

// MockPublisher is a test double for events.Publisher
type MockPublisher struct {
	events []string
}

func NewMockPublisher() *MockPublisher {
	return &MockPublisher{}
}

func (m *MockPublisher) PublishPaymentCompleted(ctx context.Context, txn *domain.Transaction, newBalance int64) {
	m.events = append(m.events, txn.ID.String())
}

func (m *MockPublisher) Close() error {
	return nil
}
