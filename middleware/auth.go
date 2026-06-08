package middleware

import (
	"acommerce_api_endpoint/models"
	"acommerce_api_endpoint/store"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireScope(required models.Scope, ts *store.TokenStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header is required"})
			c.Abort()
			return
		}
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization format must be 'Bearer <token>'"})
			c.Abort()
			return
		}
		secret := parts[1]
		token, err := ts.Get(secret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		if !token.HasScope(required) {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient scope permissions"})
			c.Abort()
			return
		}

		c.Set("Token", token)
		// c.Set("TokenSecret", secret)

		c.Next()

	}
}
