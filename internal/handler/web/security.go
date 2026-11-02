package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

const SessionCookie = "libratrack_session"
const CSRFCookie = "libratrack_csrf"

func (h *Handler) cookie(c *gin.Context, name, value string, age int) {
	http.SetCookie(c.Writer, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: age,
		HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode})
}

func (h *Handler) signature(nonce, session string) string {
	mac := hmac.New(sha256.New, h.csrfKey)
	_, _ = mac.Write([]byte(nonce + "|" + session))
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *Handler) validCSRF(token, session string) bool {
	nonce, signature, ok := strings.Cut(token, ".")
	if !ok || len(nonce) != 64 {
		return false
	}
	return hmac.Equal([]byte(signature), []byte(h.signature(nonce, session)))
}

func (h *Handler) security() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
		c.Header("Referrer-Policy", "same-origin")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		raw, _ := c.Cookie(SessionCookie)
		if raw != "" {
			if claims, err := h.tokens.Parse(raw); err == nil {
				user, err := h.repos.Users.FindByID(c.Request.Context(), claims.UserID)
				if err != nil && !errors.Is(err, repository.ErrNotFound) {
					h.fail(c, 503, "Сервіс тимчасово недоступний.")
					c.Abort()
					return
				}
				if err == nil && user.Role == claims.Role {
					c.Set("web_user", user)
				}
			}
		}
		token, _ := c.Cookie(CSRFCookie)
		safe := c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead
		if safe {
			if !h.validCSRF(token, raw) {
				var nonce [32]byte
				if _, err := rand.Read(nonce[:]); err != nil {
					h.fail(c, 500, "Не вдалося захистити форму.")
					c.Abort()
					return
				}
				value := hex.EncodeToString(nonce[:])
				token = value + "." + h.signature(value, raw)
				h.cookie(c, CSRFCookie, token, 86400)
			}
		} else {
			supplied := c.GetHeader("X-CSRF-Token")
			if supplied == "" {
				supplied = c.PostForm("csrf_token")
			}
			if !h.validCSRF(token, raw) || !hmac.Equal([]byte(supplied), []byte(token)) {
				h.fail(c, 403, "Перевірка CSRF не пройшла. Оновіть сторінку й повторіть дію.")
				c.Abort()
				return
			}
		}
		c.Set("csrf", token)
		c.Next()
	}
}

func currentUser(c *gin.Context) *model.User {
	if value, ok := c.Get("web_user"); ok {
		user := value.(model.User)
		return &user
	}
	return nil
}

func staff(user *model.User) bool {
	return user != nil && (user.Role == model.RoleLibrarian || user.Role == model.RoleAdmin)
}

func (h *Handler) requireStaff() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !staff(currentUser(c)) {
			h.fail(c, 403, "Ця дія доступна бібліотекарю або адміністратору.")
			c.Abort()
			return
		}
		c.Next()
	}
}
