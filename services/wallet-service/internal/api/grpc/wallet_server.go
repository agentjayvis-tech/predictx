// Package grpc implements the WalletService gRPC server.
// Generated proto code (walletpb) is produced by running:
//
//	protoc --go_out=. --go-grpc_out=. proto/wallet.proto
//
// Until protoc is run, this file references the expected package path.
package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/predictx/wallet-service/internal/domain"
	"github.com/predictx/wallet-service/internal/service"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	// walletpb is generated from proto/wallet.proto via:
	//   protoc --go_out=. --go-grpc_out=. proto/wallet.proto
	walletpb "github.com/predictx/wallet-service/internal/api/grpc/walletpb"
)

// WalletGRPCServer implements walletpb.WalletServiceServer.
type WalletGRPCServer struct {
	walletpb.UnimplementedWalletServiceServer
	svc *service.WalletService
	log *zap.Logger
}

func NewWalletGRPCServer(svc *service.WalletService, log *zap.Logger) *WalletGRPCServer {
	return &WalletGRPCServer{svc: svc, log: log}
}

// CheckBalance returns whether userID has sufficient balance.
// Always reads from PostgreSQL (no cache) for financial accuracy.
func (s *WalletGRPCServer) CheckBalance(ctx context.Context, req *walletpb.CheckBalanceRequest) (*walletpb.CheckBalanceResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	sufficient, balance, err := s.svc.CheckBalance(ctx, userID, domain.Currency(req.Currency), req.Amount)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &walletpb.CheckBalanceResponse{
		Sufficient:     sufficient,
		CurrentBalance: balance,
	}, nil
}

// Debit decrements user balance (bet placement, fee).
func (s *WalletGRPCServer) Debit(ctx context.Context, req *walletpb.DebitRequest) (*walletpb.TransactionResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	var refID *uuid.UUID
	if req.ReferenceId != "" {
		id, err := uuid.Parse(req.ReferenceId)
		if err == nil {
			refID = &id
		}
	}

	txn, err := s.svc.Spend(ctx, userID,
		domain.Currency(req.Currency), req.AmountMinor,
		req.IdempotencyKey, req.Description, refID, req.ReferenceType)
	if err != nil {
		if errors.Is(err, domain.ErrDuplicateTxn) {
			// Idempotency hit — return existing result.
			return &walletpb.TransactionResponse{AlreadyProcessed: true}, nil
		}
		return nil, toGRPCError(err)
	}

	// Fetch updated balance.
	balance, _ := s.svc.GetBalance(ctx, userID, domain.Currency(req.Currency))
	return &walletpb.TransactionResponse{
		TransactionId:  txn.ID.String(),
		NewBalanceMinor: balance,
	}, nil
}

// Credit increments user balance (payout, refund, daily reward).
func (s *WalletGRPCServer) Credit(ctx context.Context, req *walletpb.CreditRequest) (*walletpb.TransactionResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	var refID *uuid.UUID
	if req.ReferenceId != "" {
		id, err := uuid.Parse(req.ReferenceId)
		if err == nil {
			refID = &id
		}
	}

	currency := domain.Currency(req.Currency)
	txnType := domain.TxnType(req.TxnType)

	var (
		txn    *domain.Transaction
		txnErr error
	)
	switch txnType {
	case domain.TxnTypePayout:
		txn, txnErr = s.svc.Payout(ctx, userID, currency, req.AmountMinor, req.IdempotencyKey, req.Description, refID)
	case domain.TxnTypeRefund:
		txn, txnErr = s.svc.Refund(ctx, userID, currency, req.AmountMinor, req.IdempotencyKey, req.Description, refID)
	default:
		txn, txnErr = s.svc.Deposit(ctx, userID, currency, req.AmountMinor, req.IdempotencyKey, req.Description)
	}

	if txnErr != nil {
		if errors.Is(txnErr, domain.ErrDuplicateTxn) {
			return &walletpb.TransactionResponse{AlreadyProcessed: true}, nil
		}
		return nil, toGRPCError(txnErr)
	}

	balance, _ := s.svc.GetBalance(ctx, userID, currency)
	return &walletpb.TransactionResponse{
		TransactionId:   txn.ID.String(),
		NewBalanceMinor: balance,
	}, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInsufficientFunds):
		return status.Errorf(codes.FailedPrecondition, "%v", err)
	case errors.Is(err, domain.ErrWalletNotFound):
		return status.Errorf(codes.NotFound, "%v", err)
	case errors.Is(err, domain.ErrWalletFrozen):
		return status.Errorf(codes.PermissionDenied, "%v", err)
	case errors.Is(err, domain.ErrInvalidAmount):
		return status.Errorf(codes.InvalidArgument, "%v", err)
	case errors.Is(err, domain.ErrUnsupportedCurrency):
		return status.Errorf(codes.InvalidArgument, "%v", err)
	default:
		return status.Errorf(codes.Internal, "internal error")
	}
}
