package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/predictx/order-service/internal/api/grpc/orderpb"
	"github.com/predictx/order-service/internal/domain"
	"github.com/predictx/order-service/internal/service"
)

// OrderGRPCServer implements the gRPC OrderService.
type OrderGRPCServer struct {
	svc *service.OrderService
	log *zap.Logger
	// Embed unimplemented server for forward compatibility
	orderpb.UnimplementedOrderServiceServer
}

// NewOrderGRPCServer creates a new gRPC server.
func NewOrderGRPCServer(svc *service.OrderService, log *zap.Logger) *OrderGRPCServer {
	return &OrderGRPCServer{
		svc: svc,
		log: log,
	}
}

// CreateOrder creates a new order.
func (s *OrderGRPCServer) CreateOrder(ctx context.Context, req *orderpb.CreateOrderRequest) (*orderpb.OrderResponse, error) {
	// Validate request
	if req.UserId == "" || req.MarketId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user_id and market_id required")
	}

	serviceReq := &service.CreateOrderRequest{
		UserID:         req.UserId,
		MarketID:       req.MarketId,
		OrderType:      req.OrderType,
		TimeInForce:    req.TimeInForce,
		PriceMinor:     req.PriceMinor,
		QuantityShares: req.QuantityShares,
		Currency:       req.Currency,
		OutcomeIndex:   req.OutcomeIndex,
		IdempotencyKey: req.IdempotencyKey,
	}

	order, err := s.svc.CreateOrder(ctx, serviceReq)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return orderToProto(order), nil
}

// GetOrder retrieves an order by ID.
func (s *OrderGRPCServer) GetOrder(ctx context.Context, req *orderpb.GetOrderRequest) (*orderpb.OrderResponse, error) {
	if req.OrderId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "order_id required")
	}

	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order_id: %v", err)
	}

	order, err := s.svc.GetOrder(ctx, orderID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return orderToProto(order), nil
}

// ListUserOrders lists orders for a user.
func (s *OrderGRPCServer) ListUserOrders(req *orderpb.ListUserOrdersRequest, stream orderpb.OrderService_ListUserOrdersServer) error {
	if req.UserId == "" {
		return status.Errorf(codes.InvalidArgument, "user_id required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	var statusFilter domain.OrderStatus
	if req.StatusFilter != "" {
		statusFilter = domain.OrderStatus(req.StatusFilter)
	}

	orders, err := s.svc.ListUserOrders(stream.Context(), userID, statusFilter, int(req.Limit), int(req.Offset))
	if err != nil {
		return toGRPCError(err)
	}

	for _, order := range orders {
		if err := stream.Send(orderToProto(order)); err != nil {
			return err
		}
	}

	return nil
}

// CancelOrder cancels an order.
func (s *OrderGRPCServer) CancelOrder(ctx context.Context, req *orderpb.CancelOrderRequest) (*orderpb.OrderResponse, error) {
	if req.OrderId == "" || req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "order_id and user_id required")
	}

	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order_id: %v", err)
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	order, err := s.svc.CancelOrder(ctx, orderID, userID, req.IdempotencyKey)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return orderToProto(order), nil
}

// CheckRGLimit checks responsible gambling limits.
func (s *OrderGRPCServer) CheckRGLimit(ctx context.Context, req *orderpb.CheckRGLimitRequest) (*orderpb.CheckRGLimitResponse, error) {
	if req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user_id required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	dailyRemaining, weeklyRemaining, err := s.svc.CheckRGLimit(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &orderpb.CheckRGLimitResponse{
		WithinDailyLimit:  dailyRemaining >= req.AdditionalSpendMinor,
		WithinWeeklyLimit: weeklyRemaining >= req.AdditionalSpendMinor,
		DailyRemainingMinor:  dailyRemaining,
		WeeklyRemainingMinor: weeklyRemaining,
	}, nil
}

// Helper functions

func orderToProto(order *domain.Order) *orderpb.OrderResponse {
	return &orderpb.OrderResponse{
		OrderId:        order.ID.String(),
		UserId:         order.UserID.String(),
		MarketId:       order.MarketID.String(),
		OrderType:      string(order.OrderType),
		Status:         string(order.Status),
		PriceMinor:     order.PriceMinor,
		QuantityShares: order.QuantityShares,
		Currency:       order.Currency,
		OutcomeIndex:   int32(order.OutcomeIndex),
		CreatedAt:      order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrOrderNotFound):
		return status.Errorf(codes.NotFound, "order not found")
	case errors.Is(err, domain.ErrInvalidMarket):
		return status.Errorf(codes.FailedPrecondition, "market invalid or not active")
	case errors.Is(err, domain.ErrOutcomeIndexOutOfBounds):
		return status.Errorf(codes.InvalidArgument, "outcome_index out of bounds")
	case errors.Is(err, domain.ErrInsufficientBalance):
		return status.Errorf(codes.FailedPrecondition, "insufficient balance")
	case errors.Is(err, domain.ErrRGDailyLimitExceeded):
		return status.Errorf(codes.FailedPrecondition, "daily responsible gambling limit exceeded")
	case errors.Is(err, domain.ErrRGWeeklyLimitExceeded):
		return status.Errorf(codes.FailedPrecondition, "weekly responsible gambling limit exceeded")
	case errors.Is(err, domain.ErrRateLimitExceeded):
		return status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
	case errors.Is(err, domain.ErrInvalidTransition):
		return status.Errorf(codes.FailedPrecondition, "invalid order status transition")
	default:
		return status.Errorf(codes.Internal, "internal error")
	}
}
