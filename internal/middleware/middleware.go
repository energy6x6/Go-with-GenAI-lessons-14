package middleware

import (
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"libratrack/internal/model"
)

func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v\n%s", err, debug.Stack())
				c.AbortWithStatusJSON(500, model.ErrorResponse{Error: model.ErrorBody{Code: "internal_server_error", Message: "Internal server error"}})
			}
		}()
		c.Next()
	}
}

func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		defer func() {
			if err := recover(); err != nil {
				log.Printf("%s %s status=500 duration=%s", c.Request.Method, c.Request.URL.Path, time.Since(start))
				panic(err)
			}
			log.Printf("%s %s status=%d duration=%s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))
		}()
		c.Next()
	}
}

func CORSMiddleware() gin.HandlerFunc {
	origin := os.Getenv("ALLOWED_ORIGIN")
	if origin == "" {
		origin = "http://localhost:5173"
	}
	return func(c *gin.Context) {
		c.Header("Vary", "Origin")
		if c.GetHeader("Origin") == origin && origin != "*" {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
