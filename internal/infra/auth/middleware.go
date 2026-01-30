package auth

import (
	"context"
	"net/http"

	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
	"go.uber.org/zap"
)

// TokenValidator — интерфейс, который должны реализовать и шлюз, и консоль
type TokenValidator interface {
	VerifyToken(tokenStr string) (*domain.CustomClaims, error)
}

func NewMiddleware(v TokenValidator, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := v.VerifyToken(authHeader)
			if err != nil {
				logger.Warn("auth failure", zap.Error(err))
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Прокидываем данные в контекст
			ctx := context.WithValue(r.Context(), "user_scopes", claims.Scopes)
			ctx = context.WithValue(ctx, "user_id", claims.UserID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
