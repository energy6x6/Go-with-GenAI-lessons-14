package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"libratrack/internal/app"
	"libratrack/internal/auth"
	"libratrack/internal/chat"
	webhandler "libratrack/internal/handler/web"
	"libratrack/internal/model"
)

func chatSetup(t *testing.T) (fixture, *chat.Hub, *httptest.Server) {
	t.Helper()
	f := setup(t)
	h := chat.New(chat.Options{})
	t.Cleanup(h.Close)
	tokens, err := auth.NewTokensFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	f.router, err = app.NewRouterWithChat(f.repos, tokens, h)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(f.router)
	t.Cleanup(s.Close)
	return f, h, s
}

func webSocket(t *testing.T, f fixture, s *httptest.Server, role, origin, room string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	headers := http.Header{"Origin": {origin}}
	if role != "" {
		headers.Set("Cookie", f.sessions[role].String())
	}
	return websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http")+"/ws/chat?room="+room, headers)
}

func TestChatPageAndSocketRBAC(t *testing.T) {
	f, _, s := chatSetup(t)
	for _, role := range []string{"", model.RoleReader} {
		var cookies []*http.Cookie
		if role != "" {
			cookies = append(cookies, f.sessions[role])
		}
		for _, path := range []string{"/chat", "/chat/stats"} {
			status(t, request(f, "GET", path, nil, cookies, false), 403)
		}
		c, r, err := webSocket(t, f, s, role, s.URL, "general")
		if c != nil {
			c.Close()
		}
		if err == nil || r == nil {
			t.Fatal("unauthorized upgrade")
		}
		r.Body.Close()
		if r.StatusCode != 403 {
			t.Fatal(r.StatusCode)
		}
	}
	for _, role := range []string{model.RoleLibrarian, model.RoleAdmin} {
		out := request(f, "GET", "/chat", nil, []*http.Cookie{f.sessions[role]}, false)
		status(t, out, 200)
		if !strings.Contains(out.Body.String(), "chat.js") || !strings.Contains(out.Body.String(), "Видача книг") {
			t.Fatal("missing chat UI")
		}
		if !strings.Contains(out.Header().Get("Content-Security-Policy"), "connect-src") {
			t.Fatal("missing WebSocket CSP")
		}
		c, r, err := webSocket(t, f, s, role, s.URL, "general")
		if err != nil {
			if r != nil {
				r.Body.Close()
			}
			t.Fatal(err)
		}
		c.SetReadDeadline(time.Now().Add(time.Second))
		var e chat.Event
		if err := c.ReadJSON(&e); err != nil {
			t.Fatal(err)
		}
		c.Close()
	}
}

func TestChatUsesDatabaseIdentityAndLogoutClosesSocket(t *testing.T) {
	f, h, s := chatSetup(t)
	c, r, err := webSocket(t, f, s, model.RoleLibrarian, s.URL, "general")
	if err != nil {
		if r != nil {
			r.Body.Close()
		}
		t.Fatal(err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var e chat.Event
	if err := c.ReadJSON(&e); err != nil {
		t.Fatal(err)
	}
	c.WriteJSON(map[string]string{"text": "Library staff message", "author": "administrator", "room": "loans"})
	if err := c.ReadJSON(&e); err != nil {
		t.Fatal(err)
	}
	if e.Author != "librarian@example.com" || e.UserID != f.users[model.RoleLibrarian].ID || e.Room != "general" {
		t.Fatal("spoofed identity", e)
	}
	out := request(f, "GET", "/chat/stats", nil, []*http.Cookie{f.sessions[model.RoleLibrarian]}, false)
	status(t, out, 200)
	var stats struct {
		Chat chat.Stats `json:"chat"`
	}
	if err := json.Unmarshal(out.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Chat.Active != 1 || stats.Chat.Messages != 1 {
		t.Fatal(stats)
	}
	cookies, csrf := session(t, f, model.RoleLibrarian)
	status(t, request(f, "POST", "/logout", url.Values{"csrf_token": {csrf}}, cookies, false), 303)
	_, _, err = c.ReadMessage()
	if err == nil {
		t.Fatal("logout kept WebSocket open")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s, err := h.Stats(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if s.Active == 0 && s.ReadPumps == 0 && s.WritePumps == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("logout did not release connection and pumps")
}

func TestChatRejectsInvalidCookieOriginAndChangedRole(t *testing.T) {
	f, _, s := chatSetup(t)
	for _, origin := range []string{"https://evil.example", ""} {
		c, r, err := webSocket(t, f, s, model.RoleLibrarian, origin, "general")
		if c != nil {
			c.Close()
		}
		if err == nil || r == nil {
			t.Fatal("bad origin accepted")
		}
		r.Body.Close()
		if r.StatusCode != 403 {
			t.Fatal(r.StatusCode)
		}
	}
	f.sessions[model.RoleAdmin] = &http.Cookie{Name: webhandler.SessionCookie, Value: "invalid"}
	c, r, err := webSocket(t, f, s, model.RoleAdmin, s.URL, "general")
	if c != nil {
		c.Close()
	}
	if err == nil || r == nil {
		t.Fatal("invalid cookie accepted")
	}
	r.Body.Close()
	if r.StatusCode != 403 {
		t.Fatal(r.StatusCode)
	}
	if _, err := f.repos.Users.SetRole(t.Context(), f.users[model.RoleLibrarian].ID, model.RoleReader); err != nil {
		t.Fatal(err)
	}
	c, r, err = webSocket(t, f, s, model.RoleLibrarian, s.URL, "general")
	if c != nil {
		c.Close()
	}
	if err == nil || r == nil {
		t.Fatal("changed role accepted")
	}
	r.Body.Close()
	if r.StatusCode != 403 {
		t.Fatal(r.StatusCode)
	}
}
