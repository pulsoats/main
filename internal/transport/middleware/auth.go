package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pulsoats/main/internal/domain/auth"
	"github.com/pulsoats/main/internal/ports"
)

const (
	ContextUserIDKey    = "user_id"
	ContextRoleKey      = "role"
	ContextSessionIDKey = "session_id"
)

func AuthMiddleware(tokenService ports.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
		if tokenString == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		claims, err := tokenService.ParseAccessToken(tokenString)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextRoleKey, claims.Role)
		c.Set(ContextSessionIDKey, claims.SessionID)

		c.Next()
	}
}

func AdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := GetRole(c)
		if !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if role != "admin" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}

func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ContextUserIDKey)
	if !ok {
		return uuid.Nil, false
	}

	userID, ok := v.(uuid.UUID)
	return userID, ok
}

func GetRole(c *gin.Context) (auth.UserRole, bool) {
	v, ok := c.Get(ContextRoleKey)
	if !ok {
		return "", false
	}

	role, ok := v.(auth.UserRole)

	return role, ok
}

func GetSessionID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ContextSessionIDKey)
	if !ok {
		return uuid.Nil, false
	}

	sessionID, ok := v.(uuid.UUID)
	return sessionID, ok
}
