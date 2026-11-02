package chat

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func testHub(t *testing.T, o Options) *Hub {
	t.Helper()
	o.Logger = log.New(io.Discard, "", 0)
	h := New(o)
	t.Cleanup(h.Close)
	return h
}

func testServer(t *testing.T, h *Hub, expires time.Time) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.Serve(w, r, Identity{UserID: 1, Name: "Librarian", Session: SessionID("session"), ExpiresAt: expires}, r.URL.Query().Get("room"))
	}))
	t.Cleanup(s.Close)
	return s
}

func dial(t *testing.T, s *httptest.Server, room string) *websocket.Conn {
	t.Helper()
	c, r, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http")+"/?room="+room, http.Header{"Origin": {s.URL}})
	if err != nil {
		if r != nil {
			r.Body.Close()
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func receive(t *testing.T, c *websocket.Conn, kind string) Event {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var e Event
		if err := c.ReadJSON(&e); err != nil {
			t.Fatal(err)
		}
		if e.Type == kind {
			return e
		}
	}
}

func idle(t *testing.T, h *Hub) Stats {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s, err := h.Stats(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if s.Active == 0 && s.ReadPumps == 0 && s.WritePumps == 0 {
			return s
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("connections or pumps did not return to zero")
	return Stats{}
}

func TestThreeClientsRoomIsolationAndAuthor(t *testing.T) {
	h := testHub(t, Options{})
	s := testServer(t, h, time.Now().Add(time.Hour))
	a, b, c := dial(t, s, "general"), dial(t, s, "general"), dial(t, s, "general")
	other := dial(t, s, "loans")
	receive(t, other, "presence")
	if err := a.WriteJSON(map[string]string{"text": "Hello team", "author": "forged", "room": "loans"}); err != nil {
		t.Fatal(err)
	}
	var id uint64
	for _, conn := range []*websocket.Conn{a, b, c} {
		e := receive(t, conn, "message")
		if e.Text != "Hello team" || e.Author != "Librarian" || e.Room != "general" || e.UserID != 1 || e.Online != 3 {
			t.Fatalf("wrong event: %+v", e)
		}
		if id != 0 && id != e.ID {
			t.Fatal("different broadcast IDs")
		}
		id = e.ID
	}
	// A same-room marker must be the next message, proving no duplicate echo.
	a.WriteJSON(map[string]string{"text": "Second"})
	for _, conn := range []*websocket.Conn{a, b, c} {
		if e := receive(t, conn, "message"); e.Text != "Second" {
			t.Fatal("duplicate message", e)
		}
	}
	// A marker in the other room must arrive first there, without leaked messages.
	other.WriteJSON(map[string]string{"text": "Private room marker"})
	if e := receive(t, other, "message"); e.Text != "Private room marker" {
		t.Fatal("cross-room leakage", e)
	}
	for _, conn := range []*websocket.Conn{a, b, c, other} {
		conn.Close()
	}
	summary := idle(t, h)
	if summary.Messages != 3 || summary.Connections != 4 {
		t.Fatal(summary)
	}
}

func TestRepeatedConnectDisconnectCycles(t *testing.T) {
	h := testHub(t, Options{MaxPerUser: 128})
	s := testServer(t, h, time.Now().Add(time.Hour))
	baseline := runtime.NumGoroutine()
	for round := 0; round < 10; round++ {
		var wg sync.WaitGroup
		failures := make(chan error, 10)
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				c, r, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http")+"/?room=general", http.Header{"Origin": {s.URL}})
				if err != nil {
					if r != nil {
						r.Body.Close()
					}
					failures <- err
					return
				}
				defer c.Close()
				c.SetReadDeadline(time.Now().Add(3 * time.Second))
				var e Event
				if err := c.ReadJSON(&e); err != nil {
					failures <- err
					return
				}
				if i%2 == 0 {
					c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(1000, "cycle"), time.Now().Add(time.Second))
				}
			}(i)
		}
		wg.Wait()
		close(failures)
		for err := range failures {
			t.Fatal(err)
		}
		idle(t, h)
	}
	summary := idle(t, h)
	if summary.Connections != 100 {
		t.Fatal(summary)
	}
	// Pump counters and Close's WaitGroup are the primary lifecycle assertions.
	// Runtime counts also include the HTTP server and test runner.
	time.Sleep(30 * time.Millisecond)
	if n := runtime.NumGoroutine(); n > baseline+8 {
		t.Fatalf("goroutines baseline=%d after=%d", baseline, n)
	}
	t.Logf("100 connections; active=0 read_pumps=0 write_pumps=0; goroutines baseline=%d after=%d", baseline, runtime.NumGoroutine())
}

func TestInvalidMessagesCloseAndCleanup(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind int
		text string
		code int
	}{
		{"malformed JSON", websocket.TextMessage, "{", 1007},
		{"empty", websocket.TextMessage, `{"text":"  "}`, 1008},
		{"too many characters", websocket.TextMessage, `{"text":"` + strings.Repeat("я", 2001) + `"}`, 1008},
		{"read limit", websocket.TextMessage, `{"text":"` + strings.Repeat("a", 17000) + `"}`, 1009},
		{"binary", websocket.BinaryMessage, "hello", 1003},
		{"invalid UTF8", websocket.TextMessage, string([]byte{0xff}), 1007},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := testHub(t, Options{})
			s := testServer(t, h, time.Now().Add(time.Hour))
			c := dial(t, s, "general")
			receive(t, c, "presence")
			if err := c.WriteMessage(tc.kind, []byte(tc.text)); err != nil {
				t.Fatal(err)
			}
			c.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, _, err := c.ReadMessage()
			if !websocket.IsCloseError(err, tc.code) {
				t.Fatalf("want close %d: %v", tc.code, err)
			}
			idle(t, h)
		})
	}
}

func TestOriginAndUnknownRoom(t *testing.T) {
	h := testHub(t, Options{})
	s := testServer(t, h, time.Now().Add(time.Hour))
	for _, tc := range []struct {
		origin, room string
		status       int
	}{
		{"https://evil.example", "general", 403}, {"", "general", 403}, {s.URL + "/path", "general", 403}, {s.URL, "missing", 400},
	} {
		c, r, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http")+"/?room="+tc.room, http.Header{"Origin": {tc.origin}})
		if c != nil {
			c.Close()
		}
		if err == nil || r == nil {
			t.Fatal("unexpected upgrade", err)
		}
		r.Body.Close()
		if r.StatusCode != tc.status {
			t.Fatal(r.StatusCode)
		}
	}
	if stats := idle(t, h); stats.Connections != 0 {
		t.Fatal(stats)
	}
}

func TestHeartbeatExpiryAndShutdown(t *testing.T) {
	t.Run("missing pong", func(t *testing.T) {
		h := testHub(t, Options{PongWait: 80 * time.Millisecond, PingPeriod: 20 * time.Millisecond})
		s := testServer(t, h, time.Now().Add(time.Hour))
		c := dial(t, s, "general")
		c.SetPingHandler(func(string) error { return nil })
		for {
			c.SetReadDeadline(time.Now().Add(time.Second))
			_, _, err := c.ReadMessage()
			if err != nil {
				break
			}
		}
		idle(t, h)
	})
	t.Run("session expires despite pongs", func(t *testing.T) {
		h := testHub(t, Options{PongWait: time.Second, PingPeriod: 20 * time.Millisecond})
		s := testServer(t, h, time.Now().Add(120*time.Millisecond))
		c := dial(t, s, "general")
		for {
			c.SetReadDeadline(time.Now().Add(time.Second))
			_, _, err := c.ReadMessage()
			if err != nil {
				break
			}
		}
		idle(t, h)
	})
	t.Run("shutdown joins pumps", func(t *testing.T) {
		h := testHub(t, Options{})
		s := testServer(t, h, time.Now().Add(time.Hour))
		c := dial(t, s, "general")
		receive(t, c, "presence")
		h.Close()
		h.Close()
		if h.readers.Load() != 0 || h.writers.Load() != 0 {
			t.Fatal("pump still running")
		}
		if _, err := h.Stats(context.Background()); err != ErrClosed {
			t.Fatal(err)
		}
		if err := h.DisconnectSession(context.Background(), "session"); err != ErrClosed {
			t.Fatal(err)
		}
	})
}

func TestSessionLogoutAndConnectionLimit(t *testing.T) {
	h := testHub(t, Options{MaxPerUser: 1})
	s := testServer(t, h, time.Now().Add(time.Hour))
	c := dial(t, s, "general")
	receive(t, c, "presence")
	other := dial(t, s, "general")
	other.SetReadDeadline(time.Now().Add(time.Second))
	_, _, err := other.ReadMessage()
	if !websocket.IsCloseError(err, 1013) {
		t.Fatal("expected connection limit", err)
	}
	if err := h.DisconnectSession(context.Background(), "session"); err != nil {
		t.Fatal(err)
	}
	idle(t, h)
}

func TestSlowClientCannotBlockRoom(t *testing.T) {
	h := testHub(t, Options{QueueSize: 2})
	fake := func(id uint) *client {
		c := &client{identity: Identity{UserID: id, Name: fmt.Sprint(id)}, room: "general", send: make(chan Event, 2), done: make(chan struct{}), closeTransport: func() {}}
		if err := h.register(c); err != nil {
			t.Fatal(err)
		}
		go func() { <-c.done; h.clientsWG.Done() }()
		return c
	}
	slow := fake(1)
	<-slow.send
	fast := fake(2)
	<-slow.send
	<-fast.send
	for i := 0; i < 3; i++ {
		h.publish <- publication{fast, fmt.Sprint(i)}
		select {
		case e := <-fast.send:
			if e.Type != "message" {
				t.Fatal(e)
			}
		case <-time.After(time.Second):
			t.Fatal("slow client blocked room")
		}
	}
	select {
	case <-slow.done:
	case <-time.After(time.Second):
		t.Fatal("slow client not evicted")
	}
	s, err := h.Stats(context.Background())
	if err != nil || s.Active != 1 {
		t.Fatal(s, err)
	}
	// Repeated unregister is harmless and never closes a channel twice.
	h.leave <- slow
	h.leave <- slow
	if s, err := h.Stats(context.Background()); err != nil || s.Active != 1 {
		t.Fatal("duplicate unregister affected another client", s, err)
	}
}

func TestMessageRateLimit(t *testing.T) {
	h := testHub(t, Options{})
	s := testServer(t, h, time.Now().Add(time.Hour))
	c := dial(t, s, "general")
	receive(t, c, "presence")
	for i := 0; i < 20; i++ {
		if err := c.WriteJSON(map[string]string{"text": "rate test"}); err != nil {
			t.Fatal(err)
		}
		receive(t, c, "message")
	}
	c.WriteJSON(map[string]string{"text": "over limit"})
	c.SetReadDeadline(time.Now().Add(time.Second))
	_, _, err := c.ReadMessage()
	if !websocket.IsCloseError(err, 1008) {
		t.Fatal("rate limit did not close connection", err)
	}
	idle(t, h)
}

func TestShutdownDuringConcurrentRegistrations(t *testing.T) {
	h := testHub(t, Options{MaxPerUser: 64})
	s := testServer(t, h, time.Now().Add(time.Hour))
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, r, _ := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http")+"/?room=general", http.Header{"Origin": {s.URL}})
			if c != nil {
				c.Close()
			}
			if r != nil {
				r.Body.Close()
			}
		}()
	}
	closed := make(chan struct{})
	go func() { h.Close(); close(closed) }()
	wg.Wait()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown blocked on registration")
	}
	if h.readers.Load() != 0 || h.writers.Load() != 0 {
		t.Fatal("pumps survived shutdown")
	}
}
