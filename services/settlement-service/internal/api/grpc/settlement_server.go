// Package grpc implements the SettlementService gRPC server.
// Generated proto code (settlementpb) is produced by running:
//
//	protoc --go_out=. --go-grpc_out=. proto/settlement.proto
package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/predictx/settlement-service/internal/domain"
	"github.com/predictx/settlement-service/internal/service"
	settlementpb "github.com/predictx/settlement-service/internal/api/grpc/settlementpb"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// SettlementGRPCServer implements settlementpb.SettlementServiceServer.
type SettlementGRPCServer struct {
	settlementpb.UnimplementedSettlementServiceServer
	svc *service.SettlementService
	log *zap.Logger
}

func NewSettlementGRPCServer(svc *service.SettlementService, log *zap.Logger) *SettlementGRPCServer {
	return &SettlementGRPCServer{svc: svc, log: log}
}

// GetSettlement returns the settlement record for a resolved market.
func (s *SettlementGRPCServer) GetSettlement(ctx context.Context, req *settlementpb.GetSettlementRequest) (*settlementpb.SettlementResponse, error) {
	marketID, err := uuid.Parse(req.MarketId)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "invalid market_id: %v", err)
	}

	settlement, err := s.svc.GetSettlement(ctx, marketID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &settlementpb.SettlementResponse{
		SettlementId:      settlement.ID.String(),
		MarketId:          settlement.MarketID.String(),
		Status:            string(settlement.Status),
		WinningOutcome:    int32(settlement.WinningOutcome),
		TotalPoolMinor:    settlement.TotalPoolMinor,
		InsuranceFeeMinor: settlement.InsuranceFeeMinor,
		NetPoolMinor:      settlement.NetPoolMinor,
		WinnerCount:       int32(settlement.WinnerCount),
		LoserCount:        int32(settlement.LoserCount),
		Currency:          settlement.Currency,
	}
	if settlement.SettledAt != nil {
		resp.SettledAt = settlement.SettledAt.Format(time.RFC3339)
	}
	return resp, nil
}

// GetUserPnL returns a user's P&L for a specific market.
func (s *SettlementGRPCServer) GetUserPnL(ctx context.Context, req *settlementpb.GetUserPnLRequest) (*settlementpb.UserPnLResponse, error) {
	marketID, err := uuid.Parse(req.MarketId)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "invalid market_id: %v", err)
	}
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	pnl, err := s.svc.GetUserPnL(ctx, marketID, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &settlementpb.UserPnLResponse{
		MarketId:  marketID.String(),
		UserId:    userID.String(),
		PnlMinor:  pnl,
		IsSettled: true,
	}, nil
}

// GetUserPositions streams all positions for a user.
func (s *SettlementGRPCServer) GetUserPositions(req *settlementpb.GetUserPositionsRequest, stream settlementpb.SettlementService_GetUserPositionsServer) error {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return grpcstatus.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	positions, err := s.svc.GetUserPositions(stream.Context(), userID)
	if err != nil {
		return toGRPCError(err)
	}

	for _, p := range positions {
		if err := stream.Send(&settlementpb.PositionResponse{
			PositionId:   p.ID.String(),
			UserId:       p.UserID.String(),
			MarketId:     p.MarketID.String(),
			OutcomeIndex: int32(p.OutcomeIndex),
			StakeMinor:   p.StakeMinor,
			Currency:     p.Currency,
			Status:       string(p.Status),
		}); err != nil {
			return err
		}
	}
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrSettlementNotFound):
		return grpcstatus.Errorf(codes.NotFound, "%v", err)
	case errors.Is(err, domain.ErrPositionNotFound):
		return grpcstatus.Errorf(codes.NotFound, "%v", err)
	case errors.Is(err, domain.ErrAlreadySettled):
		return grpcstatus.Errorf(codes.AlreadyExists, "%v", err)
	case errors.Is(err, domain.ErrInvalidOutcome):
		return grpcstatus.Errorf(codes.InvalidArgument, "%v", err)
	default:
		return grpcstatus.Errorf(codes.Internal, "internal error")
	}
}
