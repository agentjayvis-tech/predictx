package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/predictx/settlement-service/internal/domain"
)

// ─── MockSettlementRepo ───────────────────────────────────────────────────────

type MockSettlementRepo struct {
	mu          sync.Mutex
	positions   map[string]*domain.Position // key: userID+marketID+outcomeIndex
	settlements map[string]*domain.Settlement // key: marketID
	entries     map[string]*domain.SettlementEntry // key: entryID
	alerts      []*domain.FraudAlert
}

func NewMockSettlementRepo() *MockSettlementRepo {
	return &MockSettlementRepo{
		positions:   make(map[string]*domain.Position),
		settlements: make(map[string]*domain.Settlement),
		entries:     make(map[string]*domain.SettlementEntry),
	}
}

func posKey(userID, marketID uuid.UUID, outcomeIndex int) string {
	return fmt.Sprintf("%s:%s:%d", userID, marketID, outcomeIndex)
}

func (m *MockSettlementRepo) UpsertPosition(ctx context.Context, p *domain.Position) (*domain.Position, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := posKey(p.UserID, p.MarketID, p.OutcomeIndex)
	if existing, ok := m.positions[key]; ok {
		existing.StakeMinor += p.StakeMinor
		existing.OrderCount++
		return existing, nil
	}
	clone := *p
	m.positions[key] = &clone
	return &clone, nil
}

func (m *MockSettlementRepo) GetPosition(ctx context.Context, userID, marketID uuid.UUID, outcomeIndex int) (*domain.Position, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := posKey(userID, marketID, outcomeIndex)
	if p, ok := m.positions[key]; ok {
		clone := *p
		return &clone, nil
	}
	return nil, domain.ErrPositionNotFound
}

func (m *MockSettlementRepo) ListPositionsByMarket(ctx context.Context, marketID uuid.UUID) ([]*domain.Position, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.Position
	for _, p := range m.positions {
		if p.MarketID == marketID {
			clone := *p
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (m *MockSettlementRepo) ListUserPositions(ctx context.Context, userID uuid.UUID) ([]*domain.Position, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.Position
	for _, p := range m.positions {
		if p.UserID == userID {
			clone := *p
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (m *MockSettlementRepo) UpdatePositionStatusByMarket(ctx context.Context, marketID uuid.UUID, newStatus, expectedStatus domain.PositionStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, p := range m.positions {
		if p.MarketID == marketID && p.Status == expectedStatus {
			m.positions[k].Status = newStatus
		}
	}
	return nil
}

func (m *MockSettlementRepo) GetSettlementByMarket(ctx context.Context, marketID uuid.UUID) (*domain.Settlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.settlements[marketID.String()]; ok {
		clone := *s
		return &clone, nil
	}
	return nil, domain.ErrSettlementNotFound
}

func (m *MockSettlementRepo) CreateSettlement(ctx context.Context, s *domain.Settlement) (*domain.Settlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *s
	m.settlements[s.MarketID.String()] = &clone
	return &clone, nil
}

func (m *MockSettlementRepo) UpdateSettlementStatus(ctx context.Context, settlementID uuid.UUID, newStatus domain.SettlementStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.settlements {
		if s.ID == settlementID {
			s.Status = newStatus
			return nil
		}
	}
	return nil
}

func (m *MockSettlementRepo) CreateSettlementEntries(ctx context.Context, entries []*domain.SettlementEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range entries {
		clone := *e
		m.entries[e.ID.String()] = &clone
	}
	return nil
}

func (m *MockSettlementRepo) UpdateEntryStatus(ctx context.Context, entryID uuid.UUID, status domain.EntryStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entries[entryID.String()]; ok {
		e.Status = status
	}
	return nil
}

func (m *MockSettlementRepo) ListPendingEntries(ctx context.Context, settlementID uuid.UUID) ([]*domain.SettlementEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.SettlementEntry
	for _, e := range m.entries {
		if e.SettlementID == settlementID && e.Status == domain.EntryStatusPending {
			clone := *e
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (m *MockSettlementRepo) InsertFraudAlert(ctx context.Context, alert *domain.FraudAlert) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = append(m.alerts, alert)
	return nil
}

// ─── MockWalletClient ────────────────────────────────────────────────────────

type MockWalletClient struct {
	mu       sync.Mutex
	credits  []WalletCall
	debits   []WalletCall
	failNext bool // if true, next call returns error
}

type WalletCall struct {
	UserID         string
	Currency       string
	AmountMinor    int64
	IdempotencyKey string
	Description    string
	ReferenceID    string
}

func NewMockWalletClient() *MockWalletClient {
	return &MockWalletClient{}
}

func (m *MockWalletClient) Credit(ctx context.Context, userID, currency string, amountMinor int64, idempotencyKey, description, referenceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return fmt.Errorf("mock wallet error")
	}
	m.credits = append(m.credits, WalletCall{
		UserID: userID, Currency: currency, AmountMinor: amountMinor,
		IdempotencyKey: idempotencyKey, Description: description, ReferenceID: referenceID,
	})
	return nil
}

func (m *MockWalletClient) Debit(ctx context.Context, userID, currency string, amountMinor int64, idempotencyKey, description, referenceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.debits = append(m.debits, WalletCall{
		UserID: userID, Currency: currency, AmountMinor: amountMinor,
		IdempotencyKey: idempotencyKey, Description: description, ReferenceID: referenceID,
	})
	return nil
}

func (m *MockWalletClient) CreditCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.credits)
}

func (m *MockWalletClient) TotalCredited() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total int64
	for _, c := range m.credits {
		total += c.AmountMinor
	}
	return total
}

// ─── MockSettlementPublisher ─────────────────────────────────────────────────

type MockSettlementPublisher struct {
	mu        sync.Mutex
	completed []string // settlement IDs
	voided    []string // market IDs
}

func NewMockSettlementPublisher() *MockSettlementPublisher {
	return &MockSettlementPublisher{}
}

func (m *MockSettlementPublisher) PublishSettlementCompleted(ctx context.Context, settlementID, marketID uuid.UUID, winnerCount int, netPoolMinor int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completed = append(m.completed, settlementID.String())
}

func (m *MockSettlementPublisher) PublishSettlementVoided(ctx context.Context, marketID uuid.UUID, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.voided = append(m.voided, marketID.String())
}

func (m *MockSettlementPublisher) Close() error { return nil }

func (m *MockSettlementPublisher) CompletedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.completed)
}
