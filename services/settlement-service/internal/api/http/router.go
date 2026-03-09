package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// NewRouter wires all HTTP routes for the Settlement Service.
func NewRouter(h *SettlementHandler, log *zap.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(zapMiddleware(log))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck
	})

	// Position endpoints (internal + user-facing)
	r.Route("/v1/positions", func(r chi.Router) {
		r.Get("/users/{userID}", h.ListUserPositions)
		r.Get("/markets/{marketID}", h.ListMarketPositions)
		r.Post("/", h.RecordPosition) // internal: called by matching engine
	})

	// Settlement endpoints
	r.Route("/v1/settlements", func(r chi.Router) {
		r.Get("/markets/{marketID}", h.GetSettlement)
		r.Get("/markets/{marketID}/pnl/{userID}", h.GetUserPnL)
	})

	return r
}

func zapMiddleware(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Info("request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
			)
			next.ServeHTTP(w, r)
		})
	}
}
