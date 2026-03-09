package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/predictx/order-service/internal/domain"
	"github.com/predictx/order-service/internal/repository"
)

// Publisher publishes order events to Kafka.
type Publisher interface {
	PublishOrderPlaced(ctx context.Context, order *domain.Order) error
	PublishOrderCancelled(ctx context.Context, order *domain.Order) error
	Close() error
}

// WalletServiceClient is the gRPC interface to Wallet Service.
type WalletServiceClient interface {
	CheckBalance(ctx context.Context, userID string, currency string, amountMinor int64) (bool, int64, error)
	CheckOrderEligibility(ctx context.Context, userID string) (eligible bool, reason string, err error)
}

// MarketServiceClient is the gRPC interface to Market Service.
type MarketServiceClient interface {
	GetMarket(ctx context.Context, marketID string) (active bool, numOutcomes int, err error)
}

// OrderCache provides order caching via Redis.
type OrderCache interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, ttlSecs int)
	Invalidate(ctx context.Context, key string)
}

// OrderService orchestrates order placement and lifecycle.
type OrderService struct {
	repo      *repository.OrderRepo
	wallet    WalletServiceClient
	market    MarketServiceClient
	cache     OrderCache
	rateLimit *RateLimiter
	rgSvc     *RGService
	publisher Publisher
	log       *zap.Logger
}

// NewOrderService creates a new order service with dependency injection.
func NewOrderService(
	repo *repository.OrderRepo,
	wallet WalletServiceClient,
	market MarketServiceClient,
	cache OrderCache,
	rateLimit *RateLimiter,
	rgSvc *RGService,
	publisher Publisher,
	log *zap.Logger,
) *OrderService {
	return &OrderService{
		repo:      repo,
		wallet:    wallet,
		market:    market,
		cache:     cache,
		rateLimit: rateLimit,
		rgSvc:     rgSvc,
		publisher: publisher,
		log:       log,
	}
}

// CreateOrder validates and persists a new order.
// Validation order per the plan:
// 1. Check rate limit
// 1b. Check order eligibility (cool-off, self-exclusion)
// 2. Fetch market + validate active + bounds
// 3. Validate outcome_index
// 4. Check user balance
// 5. Check RG limits
// 6. Persist + Publish event
func (s *OrderService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*domain.Order, error) {
	// Parse UUIDs
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		s.log.Warn("invalid user_id format", zap.String("user_id", req.UserID), zap.Error(err))
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	marketID, err := uuid.Parse(req.MarketID)
	if err != nil {
		s.log.Warn("invalid market_id format", zap.String("market_id", req.MarketID), zap.Error(err))
		return nil, fmt.Errorf("invalid market_id: %w", err)
	}

	// Step 1: Check rate limit
	if err := s.rateLimit.CheckLimit(ctx, userID); err != nil {
		s.log.Warn("rate limit exceeded", zap.String("user_id", userID.String()), zap.Error(err))
		return nil, err
	}

	// Step 1b: Check order eligibility (cool-off, self-exclusion)
	eligible, reason, err := s.wallet.CheckOrderEligibility(ctx, userID.String())
	if err != nil {
		s.log.Warn("wallet service eligibility check error", zap.String("user_id", userID.String()), zap.Error(err))
		// Log but don't fail — treat as temporary service issue, not user ineligibility
	} else if !eligible {
		s.log.Warn("order ineligible",
			zap.String("user_id", userID.String()),
			zap.String("reason", reason),
		)
		return nil, fmt.Errorf("order ineligible: %s", reason)
	}

	// Step 2: Validate market exists and is active
	marketActive, numOutcomes, err := s.market.GetMarket(ctx, marketID.String())
	if err != nil {
		s.log.Warn("market service error", zap.String("market_id", marketID.String()), zap.Error(err))
		return nil, fmt.Errorf("market service: %w", err)
	}
	if !marketActive {
		s.log.Warn("market not active", zap.String("market_id", marketID.String()))
		return nil, domain.ErrInvalidMarket
	}

	// Step 3: Validate outcome_index within market bounds
	if req.OutcomeIndex < 0 || req.OutcomeIndex >= int32(numOutcomes) {
		s.log.Warn("outcome_index out of bounds",
			zap.String("market_id", marketID.String()),
			zap.Int32("outcome_index", req.OutcomeIndex),
			zap.Int("num_outcomes", numOutcomes),
		)
		return nil, domain.ErrOutcomeIndexOutOfBounds
	}

	// Step 4: Check user balance
	balanceSufficient, currentBalance, err := s.wallet.CheckBalance(ctx, userID.String(), req.Currency, req.PriceMinor)
	if err != nil {
		s.log.Warn("wallet service error", zap.String("user_id", userID.String()), zap.Error(err))
		return nil, fmt.Errorf("wallet service: %w", err)
	}
	if !balanceSufficient {
		s.log.Warn("insufficient balance",
			zap.String("user_id", userID.String()),
			zap.Int64("required", req.PriceMinor),
			zap.Int64("current", currentBalance),
		)
		return nil, domain.ErrInsufficientBalance
	}

	// Step 5: Check RG limits
	withinDaily, withinWeekly, _, _, err := s.rgSvc.CheckAndUpdate(ctx, userID, req.PriceMinor)
	if err != nil {
		s.log.Warn("rg limit check failed",
			zap.String("user_id", userID.String()),
			zap.Bool("within_daily", withinDaily),
			zap.Bool("within_weekly", withinWeekly),
			zap.Error(err),
		)
		return nil, err
	}

	// Step 6: Create and persist order
	order := &domain.Order{
		ID:             uuid.New(),
		UserID:         userID,
		MarketID:       marketID,
		OrderType:      domain.OrderType(req.OrderType),
		Status:         domain.StatusPending,
		TimeInForce:    domain.TimeInForce(req.TimeInForce),
		PriceMinor:     req.PriceMinor,
		QuantityShares: req.QuantityShares,
		Currency:       req.Currency,
		OutcomeIndex:   int(req.OutcomeIndex),
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	createdOrder, err := s.repo.CreateOrder(ctx, order)
	if err != nil {
		s.log.Error("create order failed", zap.String("user_id", userID.String()), zap.Error(err))
		return nil, err
	}

	// Check if this was an idempotency hit
	if createdOrder.ID != order.ID {
		s.log.Info("order already exists (idempotency hit)",
			zap.String("order_id", createdOrder.ID.String()),
			zap.String("idempotency_key", req.IdempotencyKey),
		)
		return createdOrder, nil // Return existing order
	}

	// Publish event (non-blocking)
	go func() {
		if err := s.publisher.PublishOrderPlaced(context.Background(), createdOrder); err != nil {
			s.log.Warn("failed to publish order.placed event",
				zap.String("order_id", createdOrder.ID.String()),
				zap.Error(err),
			)
		}
	}()

	s.log.Info("order created successfully",
		zap.String("order_id", createdOrder.ID.String()),
		zap.String("user_id", userID.String()),
		zap.String("market_id", marketID.String()),
		zap.String("order_type", string(createdOrder.OrderType)),
		zap.Int64("price_minor", createdOrder.PriceMinor),
	)

	return createdOrder, nil
}

// GetOrder retrieves an order by ID (cache-first).
func (s *OrderService) GetOrder(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("order:%s", orderID)
	if cached, ok := s.cache.Get(ctx, cacheKey); ok {
		if order, ok := cached.(*domain.Order); ok {
			return order, nil
		}
	}

	// Fetch from DB
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// Cache for future hits
	s.cache.Set(ctx, cacheKey, order, 60)

	return order, nil
}

// ListUserOrders lists orders for a user with optional status filter.
func (s *OrderService) ListUserOrders(ctx context.Context, userID uuid.UUID, statusFilter domain.OrderStatus, limit, offset int) ([]*domain.Order, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	return s.repo.ListUserOrders(ctx, userID, statusFilter, limit, offset)
}

// ListPendingOrders lists all pending orders for a market (used by market.voided consumer).
func (s *OrderService) ListPendingOrders(ctx context.Context, marketID uuid.UUID) ([]*domain.Order, error) {
	return s.repo.ListPendingOrders(ctx, marketID)
}

// CancelOrder cancels an order with idempotency.
func (s *OrderService) CancelOrder(ctx context.Context, orderID uuid.UUID, userID uuid.UUID, idempotencyKey string) (*domain.Order, error) {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		s.log.Warn("order not found", zap.String("order_id", orderID.String()), zap.Error(err))
		return nil, err
	}

	// Verify ownership
	if order.UserID != userID {
		s.log.Warn("unauthorized cancel",
			zap.String("order_id", orderID.String()),
			zap.String("user_id", userID.String()),
		)
		return nil, fmt.Errorf("unauthorized")
	}

	// Check if transition is allowed
	if !domain.IsValidTransition(order.Status, domain.StatusCancelled) {
		s.log.Warn("invalid status transition",
			zap.String("order_id", orderID.String()),
			zap.String("current_status", string(order.Status)),
			zap.String("target_status", string(domain.StatusCancelled)),
		)
		return nil, domain.ErrInvalidTransition
	}

	// Update status (optimistic concurrency)
	if err := s.repo.UpdateStatus(ctx, orderID, domain.StatusCancelled, order.Status); err != nil {
		s.log.Warn("cancel order failed", zap.String("order_id", orderID.String()), zap.Error(err))
		return nil, err
	}

	// Fetch updated order
	updatedOrder, _ := s.repo.GetOrder(ctx, orderID)

	// Invalidate cache
	cacheKey := fmt.Sprintf("order:%s", orderID)
	s.cache.Invalidate(ctx, cacheKey)

	// Publish event (non-blocking)
	go func() {
		if err := s.publisher.PublishOrderCancelled(context.Background(), updatedOrder); err != nil {
			s.log.Warn("failed to publish order.cancelled event",
				zap.String("order_id", orderID.String()),
				zap.Error(err),
			)
		}
	}()

	s.log.Info("order cancelled successfully",
		zap.String("order_id", orderID.String()),
		zap.String("user_id", userID.String()),
	)

	return updatedOrder, nil
}

// CheckRGLimit returns current RG limit status.
func (s *OrderService) CheckRGLimit(ctx context.Context, userID uuid.UUID) (int64, int64, error) {
	return s.rgSvc.GetLimits(ctx, userID)
}

// CreateOrderRequest encapsulates order creation parameters.
type CreateOrderRequest struct {
	UserID        string
	MarketID      string
	OrderType     string
	TimeInForce   string
	PriceMinor    int64
	QuantityShares int64
	Currency      string
	OutcomeIndex  int32
	IdempotencyKey string
}
