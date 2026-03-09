// Package grpc implements the MarketService gRPC server.
// Generated proto code (marketpb) is produced by running:
//
//	protoc --go_out=. --go-grpc_out=. proto/market.proto
package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/predictx/market-service/internal/domain"
	"github.com/predictx/market-service/internal/service"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	marketpb "github.com/predictx/market-service/internal/api/grpc/marketpb"
)

// MarketGRPCServer implements marketpb.MarketServiceServer.
type MarketGRPCServer struct {
	marketpb.UnimplementedMarketServiceServer
	svc *service.MarketService
	log *zap.Logger
}

func NewMarketGRPCServer(svc *service.MarketService, log *zap.Logger) *MarketGRPCServer {
	return &MarketGRPCServer{svc: svc, log: log}
}

// GetMarket retrieves a single market by ID.
func (s *MarketGRPCServer) GetMarket(ctx context.Context, req *marketpb.GetMarketRequest) (*marketpb.MarketResponse, error) {
	marketID, err := uuid.Parse(req.MarketId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid market_id: %v", err)
	}

	m, err := s.svc.GetMarket(ctx, marketID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProto(m), nil
}

// ListResolvableMarkets streams active markets past their close time.
func (s *MarketGRPCServer) ListResolvableMarkets(_ *marketpb.Empty, stream marketpb.MarketService_ListResolvableMarketsServer) error {
	markets, err := s.svc.ListResolvable(stream.Context())
	if err != nil {
		return toGRPCError(err)
	}
	for _, m := range markets {
		if err := stream.Send(toProto(m)); err != nil {
			return err
		}
	}
	return nil
}

// ListMarkets streams markets with optional filters.
func (s *MarketGRPCServer) ListMarkets(req *marketpb.ListMarketsRequest, stream marketpb.MarketService_ListMarketsServer) error {
	f := domain.ListFilters{
		Status:   domain.MarketStatus(req.Status),
		Category: domain.MarketCategory(req.Category),
		Limit:    int(req.Limit),
		Offset:   int(req.Offset),
	}
	markets, err := s.svc.ListMarkets(stream.Context(), f)
	if err != nil {
		return toGRPCError(err)
	}
	for _, m := range markets {
		if err := stream.Send(toProto(m)); err != nil {
			return err
		}
	}
	return nil
}

// UpdateMarketStatus transitions a market to a new status.
func (s *MarketGRPCServer) UpdateMarketStatus(ctx context.Context, req *marketpb.UpdateStatusRequest) (*marketpb.MarketResponse, error) {
	marketID, err := uuid.Parse(req.MarketId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid market_id: %v", err)
	}

	if err := s.svc.UpdateStatus(ctx, marketID, domain.MarketStatus(req.NewStatus)); err != nil {
		return nil, toGRPCError(err)
	}

	m, err := s.svc.GetMarket(ctx, marketID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProto(m), nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func toProto(m *domain.Market) *marketpb.MarketResponse {
	resp := &marketpb.MarketResponse{
		MarketId:           m.ID.String(),
		Title:              m.Title,
		Question:           m.Question,
		ResolutionCriteria: m.ResolutionCriteria,
		Category:           string(m.Category),
		Status:             string(m.Status),
		PoolAmountMinor:    m.PoolAmountMinor,
		Currency:           m.Currency,
		ClosesAt:           m.ClosesAt.Format(time.RFC3339),
		CreatedAt:          m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          m.UpdatedAt.Format(time.RFC3339),
	}
	return resp
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrMarketNotFound):
		return status.Errorf(codes.NotFound, "%v", err)
	case errors.Is(err, domain.ErrInvalidTransition):
		return status.Errorf(codes.FailedPrecondition, "%v", err)
	case errors.Is(err, domain.ErrInvalidCategory):
		return status.Errorf(codes.InvalidArgument, "%v", err)
	case errors.Is(err, domain.ErrClosesAtInPast):
		return status.Errorf(codes.InvalidArgument, "%v", err)
	default:
		return status.Errorf(codes.Internal, "internal error")
	}
}
