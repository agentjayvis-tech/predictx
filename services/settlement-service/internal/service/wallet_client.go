package service

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCWalletClient implements WalletClient using raw gRPC to the Wallet Service.
// In production, replace with the generated walletpb client once proto stubs are
// published to a shared package accessible to this module.
type GRPCWalletClient struct {
	conn    *grpc.ClientConn
	timeout time.Duration
}

// NewGRPCWalletClient dials the Wallet Service at addr.
func NewGRPCWalletClient(addr string, timeoutSecs int) (*GRPCWalletClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials())) //nolint:staticcheck
	if err != nil {
		return nil, fmt.Errorf("wallet_client: dial: %w", err)
	}
	return &GRPCWalletClient{
		conn:    conn,
		timeout: time.Duration(timeoutSecs) * time.Second,
	}, nil
}

// Credit sends a Credit (payout) request to the Wallet Service.
// Noop stub — wire with walletpb generated client before enabling live settlement.
func (c *GRPCWalletClient) Credit(_ context.Context, userID, currency string, amountMinor int64, idempotencyKey, description, referenceID string) error {
	// TODO: replace with walletpb.WalletServiceClient.Credit() call.
	return fmt.Errorf("wallet_client: Credit not wired (userID=%s, amount=%d, ikey=%s)", userID, amountMinor, idempotencyKey)
}

// Debit sends a Debit request to the Wallet Service.
func (c *GRPCWalletClient) Debit(_ context.Context, userID, currency string, amountMinor int64, idempotencyKey, description, referenceID string) error {
	// TODO: replace with walletpb.WalletServiceClient.Debit() call.
	return fmt.Errorf("wallet_client: Debit not wired (userID=%s, amount=%d)", userID, amountMinor)
}

// Close shuts down the gRPC connection.
func (c *GRPCWalletClient) Close() error {
	return c.conn.Close()
}
