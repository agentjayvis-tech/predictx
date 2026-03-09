package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/predictx/wallet-service/internal/cache"
	"github.com/predictx/wallet-service/internal/domain"
	"github.com/predictx/wallet-service/internal/service"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// MockRepoForHandlerTest wraps MockWalletRepo for handler testing
type MockRepoForHandlerTest struct {
	*service.MockWalletRepo
}

func TestGetBalance_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	repo := service.NewMockWalletRepo()
	balCache := cache.NewBalanceCache(redis.NewClient(&redis.Options{Addr: "localhost:6379"}), 5)
	pub := service.NewMockPublisher()
	fraud := service.NewFraudService(repo, balCache, zap.NewNop(), 10, 100000, 80)
	svc := service.NewWalletService(repo, balCache, pub, fraud, zap.NewNop())
	handler := NewWalletHandler(svc, zap.NewNop())

	userID := uuid.New()
	svc.Deposit(ctx, userID, domain.CurrencyCoins, 5000, "test:1", "initial deposit")

	// Request
	req := httptest.NewRequest("GET", "/v1/wallets/"+userID.String()+"/balance/COINS", nil)
	w := httptest.NewRecorder()

	handler.GetBalance(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if bal, ok := resp["balance_minor"].(float64); !ok || bal != 5000 {
		t.Errorf("expected balance_minor 5000, got %v", resp["balance_minor"])
	}
}

func TestDeposit_Success(t *testing.T) {
	ctx := context.Background()
	repo := service.NewMockWalletRepo()
	balCache := cache.NewBalanceCache(redis.NewClient(&redis.Options{Addr: "localhost:6379"}), 5)
	pub := service.NewMockPublisher()
	fraud := service.NewFraudService(repo, balCache, zap.NewNop(), 10, 100000, 80)
	svc := service.NewWalletService(repo, balCache, pub, fraud, zap.NewNop())
	handler := NewWalletHandler(svc, zap.NewNop())

	userID := uuid.New()
	body := `{"currency":"COINS","amount_minor":1000,"idempotency_key":"test:dep:1","description":"test"}`
	req := httptest.NewRequest("POST", "/v1/wallets/"+userID.String()+"/deposit", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.Deposit(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if _, ok := resp["transaction_id"]; !ok {
		t.Errorf("expected transaction_id in response")
	}
}

func TestSpend_InsufficientFunds(t *testing.T) {
	ctx := context.Background()
	repo := service.NewMockWalletRepo()
	balCache := cache.NewBalanceCache(redis.NewClient(&redis.Options{Addr: "localhost:6379"}), 5)
	pub := service.NewMockPublisher()
	fraud := service.NewFraudService(repo, balCache, zap.NewNop(), 10, 100000, 80)
	svc := service.NewWalletService(repo, balCache, pub, fraud, zap.NewNop())
	handler := NewWalletHandler(svc, zap.NewNop())

	userID := uuid.New()
	svc.Deposit(ctx, userID, domain.CurrencyCoins, 100, "test:1", "deposit")

	body := `{"currency":"COINS","amount_minor":500,"idempotency_key":"test:spend:1"}`
	req := httptest.NewRequest("POST", "/v1/wallets/"+userID.String()+"/spend", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.Spend(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for insufficient funds, got %d", w.Code)
	}
}

func TestListTransactions(t *testing.T) {
	ctx := context.Background()
	repo := service.NewMockWalletRepo()
	balCache := cache.NewBalanceCache(redis.NewClient(&redis.Options{Addr: "localhost:6379"}), 5)
	pub := service.NewMockPublisher()
	fraud := service.NewFraudService(repo, balCache, zap.NewNop(), 10, 100000, 80)
	svc := service.NewWalletService(repo, balCache, pub, fraud, zap.NewNop())
	handler := NewWalletHandler(svc, zap.NewNop())

	userID := uuid.New()
	svc.Deposit(ctx, userID, domain.CurrencyCoins, 1000, "test:1", "deposit")
	svc.Spend(ctx, userID, domain.CurrencyCoins, 100, "test:2", "spend", nil, "")

	req := httptest.NewRequest("GET", "/v1/wallets/"+userID.String()+"/transactions?currency=COINS&limit=20", nil)
	w := httptest.NewRecorder()

	handler.ListTransactions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if txns, ok := resp["transactions"]; !ok || txns == nil {
		t.Errorf("expected transactions in response")
	}
}
