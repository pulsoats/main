package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain/auth"
	"github.com/pulsoats/main/internal/ports"
)

type Service struct {
	secret []byte
	ttl    time.Duration
}

var _ ports.TokenService = (*Service)(nil)

func NewService(secret []byte, ttl time.Duration) (*Service, error) {
	if secret == nil || len(secret) == 0 {
		return nil, fmt.Errorf("token service: secret: %w", errorsx.ErrInvalidArgument)
	}

	if ttl <= 0 {
		return nil, fmt.Errorf("token service: time-to-live: %w", errorsx.ErrInvalidArgument)
	}

	return &Service{
		secret: secret,
		ttl:    ttl,
	}, nil
}

type accessClaims struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

func (s *Service) GenerateAccessToken(c auth.AccessTokenClaims) (string, error) {
	now := time.Now()

	claims := accessClaims{
		UserID:    c.UserID.String(),
		Role:      string(c.Role),
		SessionID: c.SessionID.String(),

		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   c.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedStr, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("generate access token: sign: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return signedStr, nil
}

func (s *Service) ParseAccessToken(tokenString string) (auth.AccessTokenClaims, error) {
	claims := &accessClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("parse access token: unexpected signing method: %w", errorsx.ErrUnauthorized)
		}
		return s.secret, nil
	})
	if err != nil {
		return auth.AccessTokenClaims{}, fmt.Errorf("parse access token: %w", errors.Join(errorsx.ErrUnauthorized, err))
	}
	if !token.Valid {
		return auth.AccessTokenClaims{}, fmt.Errorf("parse access token: token: %w", errorsx.ErrUnauthorized)
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return auth.AccessTokenClaims{}, fmt.Errorf("parse access token: user id: %w", errorsx.ErrUnauthorized)
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return auth.AccessTokenClaims{}, fmt.Errorf("parse access token: session id: %w", errorsx.ErrUnauthorized)
	}
	role, err := auth.ParseUserRole(claims.Role)
	if err != nil {
		return auth.AccessTokenClaims{}, fmt.Errorf("parse access token: role: %w", errorsx.ErrUnauthorized)
	}

	return auth.AccessTokenClaims{
		UserID:    userID,
		Role:      role,
		SessionID: sessionID,
	}, nil
}

func (s *Service) GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Service) HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
