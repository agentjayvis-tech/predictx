package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/predictx/wallet-service/internal/cache"
	"github.com/predictx/wallet-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
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
		ID:       uuid.New(),
		UserID:   userID,
		Currency: currency,
		IsActive: true,
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

func (m *MockWalletRepo) GetBalanceAtTime(ctx context.Context, userID, walletID uuid.UUID, at any) (int64, error) {
	return 0, nil
}

// Tests

func TestDepositCreatesWallet(t *testing.T) {
	ctx := context.Background()
	repo := NewMockWalletRepo()
	balCache := cache.NewBalanceCache(redis.NewClient(&redis.Options{Addr: "localhost:6379"}), 5)
	pub := NewMockPublisher()
	fraud := NewFraudService(repo, balCache, zap.NewNop(), 10, 100000, 80)
	svc := NewWalletService(repo, balCache, pub, fraud, zap.NewNop())

	userID := uuid.New()
	txn, err := svc.Deposit(ctx, userID, domain.CurrencyCoins, 1000, "test:1", "test deposit")
	if err != nil {
		t.Fatalf("Deposit failed: %v", err)
	}

	if txn.TxnType != domain.TxnTypeDeposit {
		t.Errorf("expected TxnTypeDeposit, got %v", txn.TxnType)
	}

	wallet, _ := repo.GetWallet(ctx, userID, domain.CurrencyCoins)
	if wallet.BalanceMinor != 1000 {
		t.Errorf("expected balance 1000, got %d", wallet.BalanceMinor)
	}
}

func TestSpendRequiresSufficientFunds(t *testing.T) {
	ctx := context.Background()
	repo := NewMockWalletRepo()
	balCache := cache.NewBalanceCache(redis.NewClient(&redis.Options{Addr: "localhost:6379"}), 5)
	pub := NewMockPublisher()
	fraud := NewFraudService(repo, balCache, zap.NewNop(), 10, 100000, 80)
	svc := NewWalletService(repo, balCache, pub, fraud, zap.NewNop())

	userID := uuid.New()
	svc.Deposit(ctx, userID, domain.CurrencyCoins, 100, "test:1", "deposit")

	_, err := svc.Spend(ctx, userID, domain.CurrencyCoins, 200, "test:2", "spend", nil, "")
	if err != domain.ErrInsufficientFunds {
		t.Errorf("expected ErrInsufficientFunds, got %v", err)
	}
}

func TestIdempotencyPreventsDoubleBilling(t *testing.T) {
	ctx := context.Background()
	repo := NewMockWalletRepo()
	balCache := cache.NewBalanceCache(redis.NewClient(&redis.Options{Addr: "localhost:6379"}), 5)
	pub := NewMockPublisher()
	fraud := NewFraudService(repo, balCache, zap.NewNop(), 10, 100000, 80)
	svc := NewWalletService(repo, balCache, pub, fraud, zap.NewNop())

	userID := uuid.New()
	svc.Deposit(ctx, userID, domain.CurrencyCoins, 1000, "test:1", "deposit")

	// First spend
	txn1, err := svc.Spend(ctx, userID, domain.CurrencyCoins, 100, "test:spend:1", "first spend", nil, "")
	if err != nil {
		t.Fatalf("first spend failed: %v", err)
	}

	// Same idempotency key — should not process again
	txn2, err := svc.Spend(ctx, userID, domain.CurrencyCoins, 100, "test:spend:1", "first spend", nil, "")
	if err != nil {
		t.Fatalf("second spend failed: %v", err)
	}

	if txn1.ID != txn2.ID {
		t.Errorf("expected same transaction ID for idempotent retry")
	}

	wallet, _ := repo.GetWallet(ctx, userID, domain.CurrencyCoins)
	if wallet.BalanceMinor != 900 {
		t.Errorf("expected balance 900 after one spend, got %d", wallet.BalanceMinor)
	}
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
