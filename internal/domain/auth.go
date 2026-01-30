package domain

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	UserID string          `json:"user_id"`
	Scopes map[string]bool `json:"scopes"` // "admin": true или "jira.read": true
	jwt.RegisteredClaims
}

// Secure Token Issuing
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"` // Всегда "Bearer"
	ExpiresIn   int64  `json:"expires_in"`
}

type User struct {
	ID           string          `json:"id"`
	Email        string          `json:"email"`
	Username     string          `json:"username"`
	PasswordHash string          `json:"-"` // Никогда не отправляем на фронт
	Role         string          `json:"role"`
	Scopes       map[string]bool `json:"scopes"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
