package chat

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

const MaxTextRunes = 2000

type Identity struct {
	UserID    uint
	Name      string
	Session   [32]byte
	ExpiresAt time.Time
}

type client struct {
	hub            *Hub
	conn           *websocket.Conn
	identity       Identity
	room           string
	send           chan Event
	done           chan struct{}
	closeTransport func()
}

// SameOrigin requires a browser Origin with exactly the request's scheme/host.
// Forwarded headers are deliberately ignored; terminate TLS at this server or
// configure a trusted reverse proxy before deploying behind one.
func SameOrigin(r *http.Request) bool {
	u, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return u.Scheme == scheme && strings.EqualFold(u.Host, r.Host)
}

func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, identity Identity, room string) {
	if !ValidRoom(room) {
		http.Error(w, "Unknown chat room", 400)
		return
	}
	if identity.UserID == 0 || identity.ExpiresAt.Before(time.Now()) {
		http.Error(w, "Authentication required", 401)
		return
	}
	select {
	case <-h.stop:
		http.Error(w, "Chat unavailable", 503)
		return
	default:
	}
	u := websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024, HandshakeTimeout: 5 * time.Second, CheckOrigin: SameOrigin}
	conn, err := u.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{hub: h, conn: conn, identity: identity, room: room, send: make(chan Event, h.options.QueueSize), done: make(chan struct{}), closeTransport: func() { _ = conn.Close() }}
	if err := h.register(c); err != nil {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "Chat unavailable"), time.Now().Add(time.Second))
		_ = conn.Close()
		return
	}
	defer h.clientsWG.Done()
	writerDone := make(chan struct{})
	go func() { defer close(writerDone); c.writePump() }()
	h.readers.Add(1)
	c.readPump()
	h.readers.Add(-1)
	_ = conn.Close()
	select {
	case h.leave <- c:
	case <-h.stop:
	}
	<-writerDone
}

func (c *client) readDeadline() time.Time {
	deadline := time.Now().Add(c.hub.options.PongWait)
	if c.identity.ExpiresAt.Before(deadline) {
		return c.identity.ExpiresAt
	}
	return deadline
}

func (c *client) reject(code int, reason string) {
	_ = c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(c.hub.options.WriteWait))
}

func (c *client) readPump() {
	c.conn.SetReadLimit(16 * 1024)
	_ = c.conn.SetReadDeadline(c.readDeadline())
	c.conn.SetPongHandler(func(string) error { return c.conn.SetReadDeadline(c.readDeadline()) })
	window := time.Now()
	received := 0
	for {
		kind, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if kind != websocket.TextMessage {
			c.reject(websocket.CloseUnsupportedData, "Text messages only")
			return
		}
		var in struct {
			Text string `json:"text"`
		}
		if !utf8.Valid(data) || json.Unmarshal(data, &in) != nil {
			c.reject(websocket.CloseInvalidFramePayloadData, "Invalid JSON")
			return
		}
		in.Text = strings.TrimSpace(in.Text)
		if in.Text == "" || utf8.RuneCountInString(in.Text) > MaxTextRunes {
			c.reject(websocket.ClosePolicyViolation, "Text must contain 1-2000 characters")
			return
		}
		if time.Since(window) >= time.Second {
			window = time.Now()
			received = 0
		}
		received++
		if received > 20 {
			c.reject(websocket.ClosePolicyViolation, "Message rate exceeded")
			return
		}
		// Room and author come only from the authenticated connection.
		select {
		case c.hub.publish <- publication{c, in.Text}:
		case <-c.done:
			return
		case <-c.hub.stop:
			return
		}
	}
}

func (c *client) writePump() {
	c.hub.writers.Add(1)
	defer c.hub.writers.Add(-1)
	defer c.conn.Close()
	ticker := time.NewTicker(c.hub.options.PingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case e := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.hub.options.WriteWait)); err != nil {
				return
			}
			if err := c.conn.WriteJSON(e); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.hub.options.WriteWait)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
