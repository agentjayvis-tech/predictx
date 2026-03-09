package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// NewRouter creates and returns the HTTP router with all routes and middleware.
func NewRouter(handler *OrderHandler, log *zap.Logger) http.Handler {
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(log))

	// Health check (no auth needed)
	r.Get("/health", handler.Health)

	// v1 API routes
	r.Route("/v1", func(r chi.Router) {
		// Order endpoints
		r.Post("/orders", handler.CreateOrder)
		r.Get("/orders/{orderID}", handler.GetOrder)
		r.Post("/orders/{orderID}/cancel", handler.CancelOrder)

		// User-specific endpoints
		r.Get("/users/{userID}/orders", handler.ListUserOrders)
		r.Get("/users/{userID}/rg-limits", handler.CheckRGLimit)
	})

	return r
}

// requestLogger is middleware that logs HTTP requests.
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
