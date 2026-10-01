package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"library-system/internal/models"
	"library-system/internal/service"
)

// Keys used to stash the authenticated user's info on the Gin context,
// so downstream handlers can read c.Get(ContextUserIDKey) etc.
const (
	ContextUserIDKey = "userID"
	ContextRoleKey   = "userRole"
)

// RequireAuth checks for a valid "Authorization: Bearer <token>" header.
// Attach it to any route (or route group) that needs a logged-in user.
func RequireAuth(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "expected 'Bearer <token>' Authorization header"})
			return
		}

		claims, err := authService.ParseToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextRoleKey, claims.Role)
		c.Next()
	}
}

// RequireRole must be used AFTER RequireAuth on the same route, since it
// reads the role that RequireAuth stashed on the context.
func RequireRole(role models.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		val, exists := c.Get(ContextRoleKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing role in context"})
			return
		}

		userRole, ok := val.(models.Role)
		if !ok || userRole != role {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}

		c.Next()
	}
}
