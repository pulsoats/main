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
	const op = "token service"
	if secret == nil || len(secret) == 0 {
		return nil, fmt.Errorf("%s: secret: %w", op, errorsx.ErrInvalidArgument)
	}

	if ttl <= 0 {
		return nil, fmt.Errorf("%s: time-to-live: %w", op, errorsx.ErrInvalidArgument)
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
	const op = "generate access token"
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
		return "", fmt.Errorf("%s: sign: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	return signedStr, nil
}

func (s *Service) ParseAccessToken(tokenString string) (auth.AccessTokenClaims, error) {
	const op = "parse access token"
	claims := &accessClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%s: unexpected signing method: %w", op, errorsx.ErrUnauthorized)
		}
		return s.secret, nil
	})
	if err != nil {
		return auth.AccessTokenClaims{}, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrUnauthorized, err))
	}
	if !token.Valid {
		return auth.AccessTokenClaims{}, fmt.Errorf("%s: token: %w", op, errorsx.ErrUnauthorized)
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return auth.AccessTokenClaims{}, fmt.Errorf("%s: user id: %w", op, errorsx.ErrUnauthorized)
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return auth.AccessTokenClaims{}, fmt.Errorf("%s: session id: %w", op, errorsx.ErrUnauthorized)
	}
	role, err := auth.ParseUserRole(claims.Role)
	if err != nil {
		return auth.AccessTokenClaims{}, fmt.Errorf("%s: role: %w", op, errorsx.ErrUnauthorized)
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
