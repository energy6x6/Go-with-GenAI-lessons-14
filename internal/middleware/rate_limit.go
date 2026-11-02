package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"libratrack/internal/model"
)

// AuthRateLimit uses a bounded process-local fixed window. Multi-node deployments
// need a shared limiter. Proxies must be explicitly trusted before using client IPs.
func AuthRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	type entry struct {
		count int
		until time.Time
	}
	entries := make(map[string]entry)
	var mu sync.Mutex
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		now := time.Now()
		key := c.ClientIP()
		mu.Lock()
		value := entries[key]
		if !now.Before(value.until) {
			if len(entries) >= 10000 {
				for ip, e := range entries {
					if !now.Before(e.until) {
						delete(entries, ip)
					}
				}
			}
			if len(entries) >= 10000 {
				mu.Unlock()
				tooMany(c)
				return
			}
			value = entry{until: now.Add(window)}
		}
		value.count++
		entries[key] = value
		mu.Unlock()
		if value.count > limit {
			tooMany(c)
			return
		}
		c.Next()
	}
}

func tooMany(c *gin.Context) {
	c.Header("Retry-After", "60")
	c.AbortWithStatusJSON(429, model.ErrorResponse{Error: model.ErrorBody{Code: "rate_limited", Message: "Too many authentication attempts"}})
}
