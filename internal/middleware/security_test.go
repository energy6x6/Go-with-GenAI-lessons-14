package middleware_test

import (
	"bytes"
	"log"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"libratrack/internal/middleware"
)

func TestLoggingSkipsCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	old := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(old) })
	r := gin.New()
	r.Use(middleware.LoggingMiddleware())
	r.POST("/auth/login", func(c *gin.Context) { c.Status(200) })
	req := httptest.NewRequest("POST", "/auth/login?token=query-sensitive", strings.NewReader(`{"password":"body-sensitive","email":"personal@example.com"}`))
	req.Header.Set("Authorization", "Bearer header-sensitive")
	r.ServeHTTP(httptest.NewRecorder(), req)
	for _, secret := range []string{"query-sensitive", "body-sensitive", "header-sensitive", "personal@example.com"} {
		if strings.Contains(output.String(), secret) {
			t.Fatal("logging exposed sensitive data")
		}
	}
	if !strings.Contains(output.String(), "POST /auth/login status=200") {
		t.Fatal("request metadata missing")
	}
}

func TestRequireRoleAborts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		role string
		want int
	}{{"", 401}, {"reader", 403}, {"librarian", 200}, {"admin", 200}}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			reached := false
			r := gin.New()
			r.Use(func(c *gin.Context) {
				if tc.role != "" {
					c.Set("role", tc.role)
				}
				c.Next()
			})
			r.GET("/staff", middleware.RequireRole("librarian", "admin"), func(c *gin.Context) { reached = true; c.Status(200) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/staff", nil))
			if w.Code != tc.want || reached != (tc.want == 200) {
				t.Fatal("middleware did not stop denied request", w.Code, reached)
			}
		})
	}
}

func TestAuthRateLimitCannotBeBypassedWithForwardedIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	reached := 0
	r.POST("/login", middleware.AuthRateLimit(2, time.Minute), func(c *gin.Context) { reached++; c.Status(200) })
	for i, want := range []int{200, 200, 429} {
		req := httptest.NewRequest("POST", "/login", nil)
		req.Header.Set("X-Forwarded-For", string(rune('1'+i))+".1.1.1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatal("rate limit bypassed", w.Code)
		}
	}
	if reached != 2 {
		t.Fatal("rate-limited handler executed")
	}
}

func TestCORSAllowsOnlyConfiguredOrigin(t *testing.T) {
	t.Setenv("ALLOWED_ORIGIN", "http://localhost:5173")
	r := gin.New()
	r.Use(middleware.CORSMiddleware())
	for _, origin := range []string{"http://localhost:5173", "https://untrusted.example"} {
		req := httptest.NewRequest("OPTIONS", "/api/v1/books", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		allowed := w.Header().Get("Access-Control-Allow-Origin")
		if (origin == "http://localhost:5173") != (allowed == origin) {
			t.Fatal("CORS origin policy failed")
		}
		if !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
			t.Fatal("Bearer header missing from preflight")
		}
	}
}
