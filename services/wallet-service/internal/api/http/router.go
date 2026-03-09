package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// NewRouter wires all HTTP routes for the wallet service.
func NewRouter(h *WalletHandler, log *zap.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(log))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})

	r.Route("/v1/wallets", func(r chi.Router) {
		r.Get("/{userID}/balance", h.GetAllBalances)
		r.Get("/{userID}/balance/{currency}", h.GetBalance)
		r.Post("/{userID}/deposit", h.Deposit)
		r.Post("/{userID}/spend", h.Spend)
		r.Post("/{userID}/refund", h.Refund)
		r.Post("/{userID}/payout", h.Payout)
		r.Get("/{userID}/transactions", h.ListTransactions)
	})

	return r
}

func requestLogger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Info("http request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("request_id", middleware.GetReqID(r.Context())),
			)
			next.ServeHTTP(w, r)
		})
	}
}
