package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/predictx/settlement-service/internal/domain"
	"github.com/predictx/settlement-service/internal/service"
	"go.uber.org/zap"
)

// SettlementHandler handles HTTP requests for settlement and position operations.
type SettlementHandler struct {
	svc *service.SettlementService
	log *zap.Logger
}

func NewSettlementHandler(svc *service.SettlementService, log *zap.Logger) *SettlementHandler {
	return &SettlementHandler{svc: svc, log: log}
}

// ─── Request types ────────────────────────────────────────────────────────────

type recordPositionReq struct {
	UserID       string `json:"user_id"`
	MarketID     string `json:"market_id"`
	OutcomeIndex int    `json:"outcome_index"`
	StakeMinor   int64  `json:"stake_minor"`
	Currency     string `json:"currency"`
}

// ─── Position handlers ────────────────────────────────────────────────────────

// RecordPosition creates or updates a user's market position.
// Internal endpoint consumed by the Matching Engine on order fill.
func (h *SettlementHandler) RecordPosition(w http.ResponseWriter, r *http.Request) {
	var req recordPositionReq
	if !decodeBody(w, r, &req) {
		return
	}

	userID, ok := parseUUID(w, req.UserID)
	if !ok {
		return
	}
	marketID, ok := parseUUID(w, req.MarketID)
	if !ok {
		return
	}

	if req.StakeMinor <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "stake_minor must be positive"})
		return
	}

	currency := req.Currency
	if currency == "" {
		currency = "COINS"
	}

	pos, err := h.svc.RecordPosition(r.Context(), userID, marketID, req.OutcomeIndex, req.StakeMinor, currency)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, positionResponse(pos))
}

// ListUserPositions returns all positions for a user.
func (h *SettlementHandler) ListUserPositions(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUID(w, chi.URLParam(r, "userID"))
	if !ok {
		return
	}

	positions, err := h.svc.GetUserPositions(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}

	items := make([]map[string]any, 0, len(positions))
	for _, p := range positions {
		items = append(items, positionResponse(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"positions": items, "count": len(items)})
}

// ListMarketPositions returns all positions for a specific market.
// Internal endpoint for debugging/admin.
func (h *SettlementHandler) ListMarketPositions(w http.ResponseWriter, r *http.Request) {
	marketID, ok := parseUUID(w, chi.URLParam(r, "marketID"))
	if !ok {
		return
	}

	// Re-use GetUserPositions logic via direct repo call would be cleaner but
	// for simplicity delegate through service using a marker approach.
	// For now call list on the service interface.
	_ = marketID
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "not yet implemented"})
}

// ─── Settlement handlers ──────────────────────────────────────────────────────

// GetSettlement returns the settlement record for a resolved market.
func (h *SettlementHandler) GetSettlement(w http.ResponseWriter, r *http.Request) {
	marketID, ok := parseUUID(w, chi.URLParam(r, "marketID"))
	if !ok {
		return
	}

	s, err := h.svc.GetSettlement(r.Context(), marketID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settlementResponse(s))
}

// GetUserPnL returns a user's P&L for a market after settlement.
func (h *SettlementHandler) GetUserPnL(w http.ResponseWriter, r *http.Request) {
	marketID, ok := parseUUID(w, chi.URLParam(r, "marketID"))
	if !ok {
		return
	}
	userID, ok := parseUUID(w, chi.URLParam(r, "userID"))
	if !ok {
		return
	}

	pnl, err := h.svc.GetUserPnL(r.Context(), marketID, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"market_id": marketID.String(),
		"user_id":   userID.String(),
		"pnl_minor": pnl,
	})
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func positionResponse(p *domain.Position) map[string]any {
	return map[string]any{
		"position_id":   p.ID.String(),
		"user_id":       p.UserID.String(),
		"market_id":     p.MarketID.String(),
		"outcome_index": p.OutcomeIndex,
		"stake_minor":   p.StakeMinor,
		"currency":      p.Currency,
		"status":        string(p.Status),
		"order_count":   p.OrderCount,
		"created_at":    p.CreatedAt,
		"updated_at":    p.UpdatedAt,
	}
}

func settlementResponse(s *domain.Settlement) map[string]any {
	resp := map[string]any{
		"settlement_id":       s.ID.String(),
		"market_id":           s.MarketID.String(),
		"resolution_id":       s.ResolutionID,
		"status":              string(s.Status),
		"winning_outcome":     s.WinningOutcome,
		"total_pool_minor":    s.TotalPoolMinor,
		"insurance_fee_minor": s.InsuranceFeeMinor,
		"net_pool_minor":      s.NetPoolMinor,
		"winning_stake_minor": s.WinningStakeMinor,
		"winner_count":        s.WinnerCount,
		"loser_count":         s.LoserCount,
		"currency":            s.Currency,
		"created_at":          s.CreatedAt,
		"updated_at":          s.UpdatedAt,
	}
	if s.SettledAt != nil {
		resp["settled_at"] = s.SettledAt.Format(time.RFC3339)
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

func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrSettlementNotFound), errors.Is(err, domain.ErrPositionNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrAlreadySettled):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidOutcome):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}
