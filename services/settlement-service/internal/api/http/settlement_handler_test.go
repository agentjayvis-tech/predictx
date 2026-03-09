package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/predictx/settlement-service/internal/service"
	"go.uber.org/zap"
)

func newTestHandler(t *testing.T) *SettlementHandler {
	t.Helper()
	repo := service.NewMockSettlementRepo()
	wallet := service.NewMockWalletClient()
	pub := service.NewMockSettlementPublisher()
	// FraudService with nil Redis — checkHighConcentration is skipped gracefully.
	fraud := service.NewFraudService(nil, repo, 60, 50, 1_000_000, zap.NewNop())
	svc := service.NewSettlementService(repo, wallet, pub, fraud, "platform", "COINS", 5, zap.NewNop())
	return NewSettlementHandler(svc, zap.NewNop())
}

func TestRecordPositionHandler(t *testing.T) {
	h := newTestHandler(t)
	router := chi.NewRouter()
	router.Post("/v1/positions", h.RecordPosition)

	body := map[string]any{
		"user_id":       uuid.New().String(),
		"market_id":     uuid.New().String(),
		"outcome_index": 1,
		"stake_minor":   500,
		"currency":      "COINS",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/positions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRecordPositionHandler_BadBody(t *testing.T) {
	h := newTestHandler(t)
	router := chi.NewRouter()
	router.Post("/v1/positions", h.RecordPosition)

	req := httptest.NewRequest(http.MethodPost, "/v1/positions", bytes.NewReader([]byte("not-json")))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRecordPositionHandler_ZeroStake(t *testing.T) {
	h := newTestHandler(t)
	router := chi.NewRouter()
	router.Post("/v1/positions", h.RecordPosition)

	body := map[string]any{
		"user_id":       uuid.New().String(),
		"market_id":     uuid.New().String(),
		"outcome_index": 1,
		"stake_minor":   0, // invalid
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/positions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestGetSettlement_NotFound(t *testing.T) {
	h := newTestHandler(t)
	router := chi.NewRouter()
	router.Get("/v1/settlements/markets/{marketID}", h.GetSettlement)

	req := httptest.NewRequest(http.MethodGet, "/v1/settlements/markets/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListUserPositions_Empty(t *testing.T) {
	h := newTestHandler(t)
	router := chi.NewRouter()
	router.Get("/v1/positions/users/{userID}", h.ListUserPositions)

	req := httptest.NewRequest(http.MethodGet, "/v1/positions/users/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetUserPnL_NotSettled(t *testing.T) {
	h := newTestHandler(t)
	router := chi.NewRouter()
	router.Get("/v1/settlements/markets/{marketID}/pnl/{userID}", h.GetUserPnL)

	path := "/v1/settlements/markets/" + uuid.New().String() + "/pnl/" + uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Settlement not found → 404
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 404 or 500, got %d", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	h := newTestHandler(t)
	router := NewRouter(h, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
