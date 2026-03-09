package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/predictx/order-service/internal/domain"
)

// MockOrderRepo is an in-memory mock of OrderRepo for testing.
type MockOrderRepo struct {
	orders   map[string]*domain.Order
	rgLimits map[string]*domain.RGLimit
}

// NewMockOrderRepo creates a new mock repository.
func NewMockOrderRepo() *MockOrderRepo {
	return &MockOrderRepo{
		orders:   make(map[string]*domain.Order),
		rgLimits: make(map[string]*domain.RGLimit),
	}
}

func (m *MockOrderRepo) CreateOrder(_ context.Context, order *domain.Order) (*domain.Order, error) {
	if order.ID == uuid.Nil {
		order.ID = uuid.New()
	}
	m.orders[order.ID.String()] = order
	return order, nil
}

func (m *MockOrderRepo) GetOrder(_ context.Context, orderID uuid.UUID) (*domain.Order, error) {
	if order, ok := m.orders[orderID.String()]; ok {
		return order, nil
	}
	return nil, domain.ErrOrderNotFound
}

func (m *MockOrderRepo) GetOrderByIdempotencyKey(_ context.Context, idempotencyKey string) (*domain.Order, error) {
	for _, order := range m.orders {
		if order.IdempotencyKey == idempotencyKey {
			return order, nil
		}
	}
	return nil, domain.ErrOrderNotFound
}

func (m *MockOrderRepo) ListUserOrders(_ context.Context, userID uuid.UUID, statusFilter domain.OrderStatus, limit, offset int) ([]*domain.Order, error) {
	var result []*domain.Order
	for _, order := range m.orders {
		if order.UserID == userID {
			if statusFilter == "" || order.Status == statusFilter {
				result = append(result, order)
			}
		}
	}
	return result, nil
}

func (m *MockOrderRepo) ListPendingOrders(_ context.Context, marketID uuid.UUID) ([]*domain.Order, error) {
	var result []*domain.Order
	for _, order := range m.orders {
		if order.MarketID == marketID && order.Status == domain.StatusPending {
			result = append(result, order)
		}
	}
	return result, nil
}

func (m *MockOrderRepo) UpdateStatus(_ context.Context, orderID uuid.UUID, newStatus, oldStatus domain.OrderStatus) error {
	order, ok := m.orders[orderID.String()]
	if !ok {
		return domain.ErrOrderNotFound
	}
	if order.Status != oldStatus {
		return domain.ErrInvalidTransition
	}
	order.Status = newStatus
	return nil
}

func (m *MockOrderRepo) GetRGLimit(_ context.Context, userID uuid.UUID) (*domain.RGLimit, error) {
	if limit, ok := m.rgLimits[userID.String()]; ok {
		return limit, nil
	}
	return nil, nil // Not found is okay
}

func (m *MockOrderRepo) CreateRGLimit(_ context.Context, limit *domain.RGLimit) (*domain.RGLimit, error) {
	if limit.ID == uuid.Nil {
		limit.ID = uuid.New()
	}
	m.rgLimits[limit.UserID.String()] = limit
	return limit, nil
}

func (m *MockOrderRepo) UpdateRGLimit(_ context.Context, limit *domain.RGLimit) error {
	m.rgLimits[limit.UserID.String()] = limit
	return nil
}

// MockPublisher is an in-memory mock of event publisher for testing.
type MockPublisher struct {
	PublishedEvents []map[string]interface{}
}

// NewMockPublisher creates a new mock publisher.
func NewMockPublisher() *MockPublisher {
	return &MockPublisher{
		PublishedEvents: make([]map[string]interface{}, 0),
	}
}

func (m *MockPublisher) PublishOrderPlaced(_ context.Context, order *domain.Order) error {
	m.PublishedEvents = append(m.PublishedEvents, map[string]interface{}{
		"event":   "order.placed",
		"order":   order,
	})
	return nil
}

func (m *MockPublisher) PublishOrderCancelled(_ context.Context, order *domain.Order) error {
	m.PublishedEvents = append(m.PublishedEvents, map[string]interface{}{
		"event":   "order.cancelled",
		"order":   order,
	})
	return nil
}

func (m *MockPublisher) Close() error {
	return nil
}

// MockBalanceCache is an in-memory mock of Redis balance cache for testing.
type MockBalanceCache struct {
	cache map[string]int64
}

// NewMockBalanceCache creates a new mock cache.
func NewMockBalanceCache() *MockBalanceCache {
	return &MockBalanceCache{
		cache: make(map[string]int64),
	}
}

func (m *MockBalanceCache) Increment(_ context.Context, key string, expireSecs int) (int64, error) {
	m.cache[key]++
	return m.cache[key], nil
}

func (m *MockBalanceCache) Get(_ context.Context, key string) (interface{}, bool) {
	val, ok := m.cache[key]
	return val, ok
}

func (m *MockBalanceCache) Set(_ context.Context, key string, value interface{}, ttlSecs int) {
	if intVal, ok := value.(int64); ok {
		m.cache[key] = intVal
	}
}

func (m *MockBalanceCache) Invalidate(_ context.Context, key string) {
	delete(m.cache, key)
}
