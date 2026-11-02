package web

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"libratrack/internal/chat"
)

func (h *Handler) EnableChat(hub *chat.Hub) { h.chatHub = hub }

func (h *Handler) ChatPage(c *gin.Context) {
	if h.chatHub == nil {
		h.fail(c, 503, "Чат тимчасово недоступний.")
		return
	}
	data := h.page(c, "Чат персоналу")
	data.ChatRooms = chat.Rooms()
	h.render(c, http.StatusOK, "chat", data)
}

func (h *Handler) ChatSocket(c *gin.Context) {
	if h.chatHub == nil {
		h.fail(c, 503, "Чат тимчасово недоступний.")
		return
	}
	raw, _ := c.Cookie(SessionCookie)
	claims, err := h.tokens.Parse(raw)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	u := currentUser(c)
	name := u.Email
	if u.Name != nil && *u.Name != "" {
		name = *u.Name
	}
	h.chatHub.Serve(c.Writer, c.Request, chat.Identity{UserID: u.ID, Name: name, Session: chat.SessionID(raw), ExpiresAt: claims.ExpiresAt.Time}, c.Query("room"))
}

func (h *Handler) ChatStats(c *gin.Context) {
	if h.chatHub == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	s, err := h.chatHub.Stats(c.Request.Context())
	if err != nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	c.JSON(http.StatusOK, gin.H{"chat": s, "goroutines": runtime.NumGoroutine()})
}
