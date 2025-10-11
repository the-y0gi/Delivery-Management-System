package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"delivery-management-system/internal/auth"
	"delivery-management-system/internal/utils"
)

// AuthMiddleware validates JWT and sets user info in context
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.SendError(c, http.StatusUnauthorized, "Authorization header required")
			c.Abort()
			return
		}

		// Format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			utils.SendError(c, http.StatusUnauthorized, "Invalid authorization format")
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := auth.ValidateJWT(tokenString)
		if err != nil {
			utils.SendError(c, http.StatusUnauthorized, "Invalid or expired token")
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("userID", claims.UserID)
		c.Set("userEmail", claims.Email)
		c.Set("userRole", claims.Role)

		c.Next()
	}
}

// AdminMiddleware ensures the user has admin role
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("userRole")
		if !exists {
			utils.SendError(c, http.StatusUnauthorized, "User role not found")
			c.Abort()
			return
		}

		if userRole != "admin" {
			utils.SendError(c, http.StatusForbidden, "Admin access required")
			c.Abort()
			return
		}

		c.Next()
	}
}