package middleware

import (
	"log/slog"
	"net/http"

	transport "topology-parser/internal/delivery/http"
)

func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if value := recover(); value != nil {
					logger.ErrorContext(r.Context(), "panic recovered", "panic", value)
					transport.RespondError(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
