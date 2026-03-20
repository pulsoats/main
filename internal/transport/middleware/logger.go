package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/core/lib/logx"
)

func LoggerMiddleware(logger logx.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {

		start := time.Now()

		c.Next()

		duration := time.Since(start)

		userID, _ := c.Get("user_id")

		logger.Info("http request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", duration,
			"ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
			"user_id", userID,
		)

		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				logger.Error("http error",
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"status", c.Writer.Status(),
					"duration", duration,
					"error", err.Error(),
				)
			}
		}
	}
}
