package auth

import (
	"crypto/rsa"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
)

// BaseValidator содержит общую логику проверки RS256
type BaseValidator struct {
	publicKey *rsa.PublicKey
}

func NewBaseValidator(pubKey *rsa.PublicKey) *BaseValidator {
	return &BaseValidator{publicKey: pubKey}
}

// VerifyToken реализует интерфейс auth.TokenValidator.
// Он проверяет JWT токен, подписанный асимметричным ключом RS256.
func (v *BaseValidator) VerifyToken(tokenStr string) (*domain.CustomClaims, error) {
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	tokenStr = strings.TrimSpace(tokenStr)

	token, err := jwt.ParseWithClaims(tokenStr, &domain.CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.publicKey, nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*domain.CustomClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	return claims, nil
}

// ParseRSAPublicKey превращает []byte в объект для проверки подписи
func ParseRSAPublicKey(data []byte) (*rsa.PublicKey, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("public key data is empty")
	}
	key, err := jwt.ParseRSAPublicKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	return key, nil
}

// ParseRSAPrivateKey превращает []byte в объект для подписи (только для Console)
func ParseRSAPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("private key data is empty")
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	return key, nil
}
