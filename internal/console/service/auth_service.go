package service

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type AuthProvider interface {
	GetUserByUsername(ctx context.Context, username string) (*domain.User, error)
}

type AuthService struct {
	repo       AuthProvider
	privateKey *rsa.PrivateKey
}

func NewAuthService(repo AuthProvider, privateKey *rsa.PrivateKey) *AuthService {
	return &AuthService{
		repo:       repo,
		privateKey: privateKey,
	}
}

func (s *AuthService) GenerateToken(ctx context.Context, username, password string) (*domain.TokenResponse, error) {
	// 1. Аутентификация (Источник правды — Postgres)
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil || user == nil {
		return nil, errors.New("invalid credentials")
	}

	// 2. Проверка пароля (используем bcrypt)
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// 3. Формирование Claims (Scopes берем из прав пользователя в БД)
	expiresAt := time.Now().Add(time.Hour * 24)
	claims := &domain.CustomClaims{
		UserID: user.ID,
		Scopes: user.Scopes, // Напр. map[string]bool{"admin": true}
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "spaceai-console",
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// 4. Подпись токена ЗАКРЫТЫМ КЛЮЧОМ (RS256)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	return &domain.TokenResponse{
		AccessToken: signedToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
	}, nil
}
