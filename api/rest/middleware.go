package rest

import (
	"net/http"
	"time"

	"github.com/abcfe/abcfe-node/common/logger"
)

// LoggingMiddleware HTTP request logging middleware
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Call next handler
		next.ServeHTTP(w, r)

		// Log request
		duration := time.Since(start)
		logger.Info("Request:", r.Method, r.URL.Path, "Duration:", duration)
	})
}

// RecoveryMiddleware panic recovery middleware
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("API Panic recovered:", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
