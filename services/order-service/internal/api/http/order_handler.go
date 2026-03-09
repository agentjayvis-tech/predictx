package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/predictx/order-service/internal/domain"
	"github.com/predictx/order-service/internal/service"
)

// OrderHandler handles HTTP requests for orders.
type OrderHandler struct {
	svc *service.OrderService
	log *zap.Logger
}

// NewOrderHandler creates a new order handler.
func NewOrderHandler(svc *service.OrderService, log *zap.Logger) *OrderHandler {
	return &OrderHandler{
		svc: svc,
		log: log,
	}
}

// CreateOrder POST /v1/orders
func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if !decodeBody(w, r, &req) {
		return
	}

	serviceReq := &service.CreateOrderRequest{
		UserID:         req.UserID,
		MarketID:       req.MarketID,
		OrderType:      req.OrderType,
		TimeInForce:    req.TimeInForce,
		PriceMinor:     req.PriceMinor,
		QuantityShares: req.QuantityShares,
		Currency:       req.Currency,
		OutcomeIndex:   req.OutcomeIndex,
		IdempotencyKey: req.IdempotencyKey,
	}

	order, err := h.svc.CreateOrder(r.Context(), serviceReq)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, orderResponse(order))
}

// GetOrder GET /v1/orders/{orderID}
func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	orderID, ok := parseUUID(w, chi.URLParam(r, "orderID"))
	if !ok {
		return
	}

	order, err := h.svc.GetOrder(r.Context(), orderID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, orderResponse(order))
}

// CancelOrder POST /v1/orders/{orderID}/cancel
func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	orderID, ok := parseUUID(w, chi.URLParam(r, "orderID"))
	if !ok {
		return
	}

	var req cancelOrderRequest
	if !decodeBody(w, r, &req) {
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	order, err := h.svc.CancelOrder(r.Context(), orderID, userID, req.IdempotencyKey)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, orderResponse(order))
}

// ListUserOrders GET /v1/users/{userID}/orders
func (h *OrderHandler) ListUserOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUID(w, chi.URLParam(r, "userID"))
	if !ok {
		return
	}

	statusFilter := r.URL.Query().Get("status")
	limit := parseIntQuery(r, "limit", 20, 1, 100)
	offset := parseIntQuery(r, "offset", 0, 0, 10000)

	orders, err := h.svc.ListUserOrders(r.Context(), userID, domain.OrderStatus(statusFilter), limit, offset)
	if err != nil {
		writeError(w, err)
		return
	}

	var resp []map[string]interface{}
	for _, order := range orders {
		resp = append(resp, orderResponse(order))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"orders": resp,
		"count":  len(resp),
	})
}

// CheckRGLimit GET /v1/users/{userID}/rg-limits
func (h *OrderHandler) CheckRGLimit(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUUID(w, chi.URLParam(r, "userID"))
	if !ok {
		return
	}

	dailyRemaining, weeklyRemaining, err := h.svc.CheckRGLimit(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"daily_remaining_minor":  dailyRemaining,
		"weekly_remaining_minor": weeklyRemaining,
	})
}

// Health GET /health
func (h *OrderHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"service": "order-service",
	})
}

// Helper types
type createOrderRequest struct {
	UserID         string `json:"user_id"`
	MarketID       string `json:"market_id"`
	OrderType      string `json:"order_type"`
	TimeInForce    string `json:"time_in_force"`
	PriceMinor     int64  `json:"price_minor"`
	QuantityShares int64  `json:"quantity_shares"`
	Currency       string `json:"currency"`
	OutcomeIndex   int32  `json:"outcome_index"`
	IdempotencyKey string `json:"idempotency_key"`
}

type cancelOrderRequest struct {
	UserID         string `json:"user_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

// Helper functions
func orderResponse(order *domain.Order) map[string]interface{} {
	return map[string]interface{}{
		"id":               order.ID.String(),
		"user_id":          order.UserID.String(),
		"market_id":        order.MarketID.String(),
		"order_type":       string(order.OrderType),
		"status":           string(order.Status),
		"time_in_force":    string(order.TimeInForce),
		"price_minor":      order.PriceMinor,
		"quantity_shares":  order.QuantityShares,
		"currency":         order.Currency,
		"outcome_index":    order.OutcomeIndex,
		"created_at":       order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":       order.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrOrderNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidMarket):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrInsufficientBalance):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrRGDailyLimitExceeded), errors.Is(err, domain.ErrRGWeeklyLimitExceeded):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrRateLimitExceeded):
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidTransition):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data) //nolint:errcheck
}

func decodeBody(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}

func parseUUID(w http.ResponseWriter, raw string) (uuid.UUID, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid UUID"})
		return uuid.Nil, false
	}
	return id, true
}

func parseIntQuery(r *http.Request, key string, defaultVal, minVal, maxVal int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	var val int
	if err := json.Unmarshal([]byte(raw), &val); err != nil {
		return defaultVal
	}
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}
