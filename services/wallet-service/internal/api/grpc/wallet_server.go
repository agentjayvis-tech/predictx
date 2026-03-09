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
	"time"

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

// ─── Responsible Gambling RPCs ───────────────────────────────────────────────

// GetDepositSettings returns user's deposit limit configuration.
func (s *WalletGRPCServer) GetDepositSettings(ctx context.Context, req *walletpb.GetDepositSettingsRequest) (*walletpb.DepositSettingsResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	ds, err := s.svc.GetDepositSettings(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch deposit settings: %v", err)
	}

	monthlyLimit := int64(0)
	if ds.MonthlyDepositLimitMinor != nil {
		monthlyLimit = *ds.MonthlyDepositLimitMinor
	}

	return &walletpb.DepositSettingsResponse{
		UserId:                   userID.String(),
		DailyDepositLimitMinor:   ds.DailyDepositLimitMinor,
		MonthlyDepositLimitMinor: monthlyLimit,
		Enabled:                  ds.Enabled,
		CreatedAt:                ds.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                ds.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// UpdateDepositSettings updates user's deposit limits.
func (s *WalletGRPCServer) UpdateDepositSettings(ctx context.Context, req *walletpb.UpdateDepositSettingsRequest) (*walletpb.DepositSettingsResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	monthlyLimit := (*int64)(nil)
	if req.MonthlyDepositLimitMinor > 0 {
		monthlyLimit = &req.MonthlyDepositLimitMinor
	}

	ds, err := s.svc.UpdateDepositSettings(ctx, userID, req.DailyDepositLimitMinor, monthlyLimit)
	if err != nil {
		return nil, toGRPCError(err)
	}

	monthlyLimitResp := int64(0)
	if ds.MonthlyDepositLimitMinor != nil {
		monthlyLimitResp = *ds.MonthlyDepositLimitMinor
	}

	return &walletpb.DepositSettingsResponse{
		UserId:                   userID.String(),
		DailyDepositLimitMinor:   ds.DailyDepositLimitMinor,
		MonthlyDepositLimitMinor: monthlyLimitResp,
		Enabled:                  ds.Enabled,
		CreatedAt:                ds.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                ds.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// GetExclusionSettings returns user's cool-off and self-exclusion configuration.
func (s *WalletGRPCServer) GetExclusionSettings(ctx context.Context, req *walletpb.GetExclusionSettingsRequest) (*walletpb.ExclusionSettingsResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	es, err := s.svc.GetExclusionSettings(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch exclusion settings: %v", err)
	}

	now := time.Now()
	inCoolOff := es.IsInCoolOff(now)
	coolOffRemaining := int32(0)
	if inCoolOff {
		coolOffRemaining = int32(es.CoolOffRemaining(now).Hours())
	}

	coolOffDuration := int32(0)
	if es.CoolOffDurationHours != nil {
		coolOffDuration = int32(*es.CoolOffDurationHours)
	}

	selfExclusionRemaining := int32(0)
	selfExclusionDuration := int32(0)
	if es.IsSelfExcluded && es.SelfExclusionDurationDays != nil {
		selfExclusionDuration = int32(*es.SelfExclusionDurationDays)
		if remainingPtr := es.SelfExclusionRemaining(now); remainingPtr != nil {
			selfExclusionRemaining = int32(remainingPtr.Hours() / 24)
		}
	}

	countryCode := ""
	if es.CountryCode != nil {
		countryCode = *es.CountryCode
	}

	return &walletpb.ExclusionSettingsResponse{
		UserId:                    userID.String(),
		CountryCode:               countryCode,
		InCoolOff:                 inCoolOff,
		CoolOffRemainingHours:     coolOffRemaining,
		CoolOffDurationHours:      coolOffDuration,
		CoolOffCancellable:        es.CoolOffCancellable,
		IsSelfExcluded:            es.IsSelfExcluded,
		SelfExclusionRemainingDays: selfExclusionRemaining,
		SelfExclusionDurationDays: selfExclusionDuration,
		CreatedAt:                 es.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                 es.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// StartCoolOff initiates a cool-off period.
func (s *WalletGRPCServer) StartCoolOff(ctx context.Context, req *walletpb.StartCoolOffRequest) (*walletpb.ExclusionSettingsResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	es, err := s.svc.StartCoolOff(ctx, userID, int(req.DurationHours))
	if err != nil {
		return nil, toGRPCError(err)
	}

	// Return updated settings
	return s.GetExclusionSettings(ctx, &walletpb.GetExclusionSettingsRequest{UserId: userID.String()})
}

// CancelCoolOff cancels an active cool-off period.
func (s *WalletGRPCServer) CancelCoolOff(ctx context.Context, req *walletpb.CancelCoolOffRequest) (*walletpb.ExclusionSettingsResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	_, err = s.svc.CancelCoolOff(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	// Return updated settings
	return s.GetExclusionSettings(ctx, &walletpb.GetExclusionSettingsRequest{UserId: userID.String()})
}

// SelfExclude initiates self-exclusion.
func (s *WalletGRPCServer) SelfExclude(ctx context.Context, req *walletpb.SelfExcludeRequest) (*walletpb.ExclusionSettingsResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	var durationPtr *int
	if req.DurationDays > 0 {
		durationPtr = &[]int{int(req.DurationDays)}[0]
	}

	_, err = s.svc.SelfExclude(ctx, userID, durationPtr)
	if err != nil {
		return nil, toGRPCError(err)
	}

	// Return updated settings
	return s.GetExclusionSettings(ctx, &walletpb.GetExclusionSettingsRequest{UserId: userID.String()})
}

// CheckOrderEligibility checks if user can place an order.
func (s *WalletGRPCServer) CheckOrderEligibility(ctx context.Context, req *walletpb.CheckOrderEligibilityRequest) (*walletpb.CheckOrderEligibilityResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	err = s.svc.CheckOrderEligibility(ctx, userID)
	if err == nil {
		return &walletpb.CheckOrderEligibilityResponse{
			Eligible: true,
		}, nil
	}

	reason := "ineligible"
	if errors.Is(err, domain.ErrInCoolOffPeriod) {
		reason = "in_cool_off_period"
	} else if errors.Is(err, domain.ErrSelfExcluded) {
		reason = "self_excluded"
	}

	return &walletpb.CheckOrderEligibilityResponse{
		Eligible: false,
		Reason:   reason,
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

	// Responsible Gambling errors
	case errors.Is(err, domain.ErrDepositLimitExceeded):
		return status.Errorf(codes.FailedPrecondition, "deposit_limit_exceeded")
	case errors.Is(err, domain.ErrInCoolOffPeriod):
		return status.Errorf(codes.PermissionDenied, "in_cool_off_period")
	case errors.Is(err, domain.ErrSelfExcluded):
		return status.Errorf(codes.PermissionDenied, "self_excluded")
	case errors.Is(err, domain.ErrInvalidDepositLimit):
		return status.Errorf(codes.InvalidArgument, "invalid_deposit_limit")
	case errors.Is(err, domain.ErrMonthlyLimitBelowDaily):
		return status.Errorf(codes.InvalidArgument, "monthly_limit_below_daily")
	case errors.Is(err, domain.ErrInvalidCoolOff):
		return status.Errorf(codes.InvalidArgument, "invalid_cool_off")
	case errors.Is(err, domain.ErrInvalidCoolOffDuration):
		return status.Errorf(codes.InvalidArgument, "invalid_cool_off_duration")
	case errors.Is(err, domain.ErrInvalidSelfExclusion):
		return status.Errorf(codes.InvalidArgument, "invalid_self_exclusion")
	case errors.Is(err, domain.ErrCoolOffNotCancellable):
		return status.Errorf(codes.PermissionDenied, "cool_off_not_cancellable_in_region")

	default:
		return status.Errorf(codes.Internal, "internal error")
	}
}
