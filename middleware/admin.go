package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RequireAdminKey(expectedKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expectedKey == "" {
			c.JSON(http.StatusForbidden, gin.H{"errpr": "admin access is not configured"})
			c.Abort()
			return
		}
		provided := c.GetHeader("X-Admin-Key")
		if provided == "" || provided != expectedKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing admin API key"})
			c.Abort()
			return
		}

		c.Next()
	}
}
