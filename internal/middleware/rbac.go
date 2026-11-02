package middleware

import (
	"github.com/gin-gonic/gin"
	"libratrack/internal/model"
)

func RequireRole(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get("role")
		if !exists {
			unauthenticated(c)
			return
		}
		role, ok := value.(string)
		if !ok || !model.ValidRole(role) {
			unauthenticated(c)
			return
		}
		for _, candidate := range allowed {
			if role == candidate {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(403, model.ErrorResponse{Error: model.ErrorBody{Code: "FORBIDDEN", Message: "insufficient permissions"}})
	}
}
