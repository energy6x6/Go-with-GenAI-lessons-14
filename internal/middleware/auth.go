package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"libratrack/internal/auth"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

func AuthMiddleware(tokens *auth.Tokens, users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			unauthenticated(c)
			return
		}
		claims, err := tokens.Parse(parts[1])
		if err != nil {
			unauthenticated(c)
			return
		}
		// Role changes invalidate old tokens immediately; deleted users cannot authenticate.
		user, err := users.FindByID(c.Request.Context(), claims.UserID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				unauthenticated(c)
			} else {
				c.AbortWithStatusJSON(503, model.ErrorResponse{Error: model.ErrorBody{Code: "unavailable", Message: "Service unavailable"}})
			}
			return
		}
		if user.Role != claims.Role {
			unauthenticated(c)
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func unauthenticated(c *gin.Context) {
	c.AbortWithStatusJSON(401, model.ErrorResponse{Error: model.ErrorBody{Code: "UNAUTHENTICATED", Message: "unauthenticated"}})
}
