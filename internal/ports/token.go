package ports

import (
	"github.com/pulsoats/main/internal/domain/auth"
)

// TokenService describes operations for creating and validating application tokens.
type TokenService interface {
	GenerateAccessToken(claims auth.AccessTokenClaims) (string, error)
	ParseAccessToken(tokenString string) (auth.AccessTokenClaims, error)
	GenerateToken() (string, error)
	HashToken(token string) string
}
