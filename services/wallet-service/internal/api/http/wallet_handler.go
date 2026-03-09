package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/predictx/wallet-service/internal/domain"
	"github.com/predictx/wallet-service/internal/service"
	"go.uber.org/zap"
)

// WalletHandler handles HTTP requests for wallet operations.
type WalletHandler struct {
	svc *service.WalletService
	log *zap.Logger
}

func NewWalletHandler(svc *service.WalletService, log *zap.Logger) *WalletHandler {
	return &WalletHandler{svc: svc, log: log}
}

// ─── Request / Response types ─────────────────────────────────────────────────

type transactionReq struct {
	Currency       string `json:"currency"`
	AmountMinor    int64  `json:"amount_minor"`
	IdempotencyKey string `json:"idempotency_key"`
	Description    string `json:"description"`
	ReferenceID    string `json:"reference_id,omitempty"`
	ReferenceType  string `json:"reference_type,omitempty"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// GetAllBalances returns all currency balances for a user.
func (h *WalletHandler) GetAllBalances(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUID(w, chi.URLParam(r, "userID"))
	if !ok {
		return
	}

	wallets, err := h.svc.GetAllWallets(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}

	balances := make([]map[string]any, 0, len(wallets))
	for _, wl := range wallets {
		balances = append(balances, map[string]any{
			"currency":      string(wl.Currency),
			"balance_minor": wl.BalanceMinor,
			"is_active":     wl.IsActive,
			"updated_at":    wl.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":  userID,
		"balances": balances,
	})
}

// GetBalance returns the balance for a specific currency.
func (h *WalletHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUID(w, chi.URLParam(r, "userID"))
	if !ok {
		return
	}
	currency := domain.Currency(chi.URLParam(r, "currency"))

	balance, err := h.svc.GetBalance(r.Context(), userID, currency)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":       userID,
		"currency":      string(currency),
		"balance_minor": balance,
	})
}

// Deposit credits funds to a user's wallet.
func (h *WalletHandler) Deposit(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUID(w, chi.URLParam(r, "userID"))
	if !ok {
		return
	}
	var req transactionReq
	if !decodeBody(w, r, &req) {
		return
	}

	txn, err := h.svc.Deposit(r.Context(), userID,
		domain.Currency(req.Currency), req.AmountMinor,
		req.IdempotencyKey, req.Description)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, txnResponse(txn))
}

// Spend debits funds from a user's wallet for bet placement.
func (h *WalletHandler) Spend(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUID(w, chi.URLParam(r, "userID"))
	if !ok {
		return
	}
	var req transactionReq
	if !decodeBody(w, r, &req) {
		return
	}

	var refID *uuid.UUID
	if req.ReferenceID != "" {
		id, err := uuid.Parse(req.ReferenceID)
		if err == nil {
			refID = &id
		}
	}

	txn, err := h.svc.Spend(r.Context(), userID,
		domain.Currency(req.Currency), req.AmountMinor,
		req.IdempotencyKey, req.Description, refID, req.ReferenceType)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, txnResponse(txn))
}

// Refund returns funds to a user's wallet (voided/disputed market).
func (h *WalletHandler) Refund(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUID(w, chi.URLParam(r, "userID"))
	if !ok {
		return
	}
	var req transactionReq
	if !decodeBody(w, r, &req) {
		return
	}

	var refID *uuid.UUID
	if req.ReferenceID != "" {
		id, err := uuid.Parse(req.ReferenceID)
		if err == nil {
			refID = &id
		}
	}

	txn, err := h.svc.Refund(r.Context(), userID,
		domain.Currency(req.Currency), req.AmountMinor,
		req.IdempotencyKey, req.Description, refID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, txnResponse(txn))
}

// Payout credits winnings to a user's wallet.
func (h *WalletHandler) Payout(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUID(w, chi.URLParam(r, "userID"))
	if !ok {
		return
	}
	var req transactionReq
	if !decodeBody(w, r, &req) {
		return
	}

	var refID *uuid.UUID
	if req.ReferenceID != "" {
		id, err := uuid.Parse(req.ReferenceID)
		if err == nil {
			refID = &id
		}
	}

	txn, err := h.svc.Payout(r.Context(), userID,
		domain.Currency(req.Currency), req.AmountMinor,
		req.IdempotencyKey, req.Description, refID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, txnResponse(txn))
}

// ListTransactions returns paginated transaction history.
func (h *WalletHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUID(w, chi.URLParam(r, "userID"))
	if !ok {
		return
	}
	currency := domain.Currency(r.URL.Query().Get("currency"))
	if currency == "" {
		currency = domain.CurrencyCoins
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	txns, err := h.svc.ListTransactions(r.Context(), userID, currency, limit, offset)
	if err != nil {
		writeError(w, err)
		return
	}

	items := make([]map[string]any, 0, len(txns))
	for _, t := range txns {
		items = append(items, txnResponse(t))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":      userID,
		"currency":     string(currency),
		"transactions": items,
	})
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func txnResponse(t *domain.Transaction) map[string]any {
	resp := map[string]any{
		"transaction_id":  t.ID.String(),
		"user_id":         t.UserID.String(),
		"txn_type":        string(t.TxnType),
		"status":          t.Status,
		"currency":        string(t.Currency),
		"amount_minor":    t.AmountMinor,
		"description":     t.Description,
		"reference_type":  t.ReferenceType,
		"idempotency_key": t.IdempotencyKey,
		"created_at":      t.CreatedAt,
	}
	if t.ReferenceID != nil {
		resp["reference_id"] = t.ReferenceID.String()
	}
	return resp
}

func parseUUID(w http.ResponseWriter, s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid uuid"})
		return uuid.UUID{}, false
	}
	return id, true
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInsufficientFunds):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrWalletNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrWalletFrozen):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrDuplicateTxn):
		writeJSON(w, http.StatusOK, map[string]string{"error": err.Error(), "already_processed": "true"})
	case errors.Is(err, domain.ErrInvalidAmount):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrUnsupportedCurrency):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}
