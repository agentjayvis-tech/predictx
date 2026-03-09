package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/predictx/market-service/internal/domain"
	"github.com/predictx/market-service/internal/service"
	"go.uber.org/zap"
)

// MarketHandler handles HTTP requests for market operations.
type MarketHandler struct {
	svc *service.MarketService
	log *zap.Logger
}

func NewMarketHandler(svc *service.MarketService, log *zap.Logger) *MarketHandler {
	return &MarketHandler{svc: svc, log: log}
}

// ─── Request types ────────────────────────────────────────────────────────────

type createMarketReq struct {
	Title              string         `json:"title"`
	Question           string         `json:"question"`
	ResolutionCriteria string         `json:"resolution_criteria"`
	Category           string         `json:"category"`
	CreatorID          string         `json:"creator_id"`
	CreatorType        string         `json:"creator_type"`
	ClosesAt           time.Time      `json:"closes_at"`
	ResolvesAt         *time.Time     `json:"resolves_at,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

type updateStatusReq struct {
	NewStatus string `json:"new_status"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// CreateMarket creates a new prediction market.
func (h *MarketHandler) CreateMarket(w http.ResponseWriter, r *http.Request) {
	var req createMarketReq
	if !decodeBody(w, r, &req) {
		return
	}

	creatorID, ok := parseUUID(w, req.CreatorID)
	if !ok {
		return
	}

	domainReq := domain.CreateMarketRequest{
		Title:              req.Title,
		Question:           req.Question,
		ResolutionCriteria: req.ResolutionCriteria,
		Category:           domain.MarketCategory(req.Category),
		CreatorID:          creatorID,
		CreatorType:        domain.CreatorType(req.CreatorType),
		ClosesAt:           req.ClosesAt,
		ResolvesAt:         req.ResolvesAt,
		Metadata:           req.Metadata,
	}

	m, err := h.svc.CreateMarket(r.Context(), domainReq)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, marketResponse(m))
}

// GetMarket returns a single market by ID.
func (h *MarketHandler) GetMarket(w http.ResponseWriter, r *http.Request) {
	marketID, ok := parseUUID(w, chi.URLParam(r, "marketID"))
	if !ok {
		return
	}

	m, err := h.svc.GetMarket(r.Context(), marketID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, marketResponse(m))
}

// ListMarkets returns markets with optional filters.
func (h *MarketHandler) ListMarkets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	filters := domain.ListFilters{
		Status:   domain.MarketStatus(q.Get("status")),
		Category: domain.MarketCategory(q.Get("category")),
		Limit:    limit,
		Offset:   offset,
	}

	markets, err := h.svc.ListMarkets(r.Context(), filters)
	if err != nil {
		writeError(w, err)
		return
	}

	items := make([]map[string]any, 0, len(markets))
	for _, m := range markets {
		items = append(items, marketResponse(m))
	}
	writeJSON(w, http.StatusOK, map[string]any{"markets": items, "count": len(items)})
}

// UpdateStatus transitions a market to a new status (admin only).
func (h *MarketHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	marketID, ok := parseUUID(w, chi.URLParam(r, "marketID"))
	if !ok {
		return
	}

	var req updateStatusReq
	if !decodeBody(w, r, &req) {
		return
	}

	if err := h.svc.UpdateStatus(r.Context(), marketID, domain.MarketStatus(req.NewStatus)); err != nil {
		writeError(w, err)
		return
	}

	m, err := h.svc.GetMarket(r.Context(), marketID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, marketResponse(m))
}

// ListResolvable returns active markets past their close time.
// Called by the Resolution Service every 60 seconds.
func (h *MarketHandler) ListResolvable(w http.ResponseWriter, r *http.Request) {
	markets, err := h.svc.ListResolvable(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	items := make([]map[string]any, 0, len(markets))
	for _, m := range markets {
		items = append(items, resolvableResponse(m))
	}
	writeJSON(w, http.StatusOK, items)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func marketResponse(m *domain.Market) map[string]any {
	resp := map[string]any{
		"market_id":           m.ID.String(),
		"title":               m.Title,
		"question":            m.Question,
		"resolution_criteria": m.ResolutionCriteria,
		"category":            string(m.Category),
		"status":              string(m.Status),
		"creator_id":          m.CreatorID.String(),
		"creator_type":        string(m.CreatorType),
		"pool_amount_minor":   m.PoolAmountMinor,
		"currency":            m.Currency,
		"closes_at":           m.ClosesAt,
		"metadata":            m.Metadata,
		"created_at":          m.CreatedAt,
		"updated_at":          m.UpdatedAt,
	}
	if m.ResolvesAt != nil {
		resp["resolves_at"] = m.ResolvesAt
	}
	return resp
}

// resolvableResponse returns the shape expected by Resolution Service.
func resolvableResponse(m *domain.Market) map[string]any {
	return map[string]any{
		"market_id":           m.ID.String(),
		"question":            m.Question,
		"resolution_criteria": m.ResolutionCriteria,
		"category":            string(m.Category),
		"metadata":            m.Metadata,
		"closes_at":           m.ClosesAt.Format(time.RFC3339),
	}
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
	case errors.Is(err, domain.ErrMarketNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidTransition):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidCategory), errors.Is(err, domain.ErrClosesAtInPast):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}
