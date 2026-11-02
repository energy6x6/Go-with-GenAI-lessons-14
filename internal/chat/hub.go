// Package chat owns room membership and the lifecycle of WebSocket connections.
package chat

import (
	"context"
	"crypto/sha256"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

var ErrClosed = errors.New("chat is closed")

type Room struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func Rooms() []Room {
	return []Room{{"general", "Загальна"}, {"loans", "Видача книг"}, {"catalog", "Каталог"}}
}
func ValidRoom(id string) bool {
	for _, r := range Rooms() {
		if r.ID == id {
			return true
		}
	}
	return false
}
func SessionID(raw string) [32]byte { return sha256.Sum256([]byte(raw)) }

type Event struct {
	Type   string    `json:"type"`
	Room   string    `json:"room"`
	ID     uint64    `json:"id,omitempty"`
	Author string    `json:"author,omitempty"`
	UserID uint      `json:"user_id,omitempty"`
	Text   string    `json:"text,omitempty"`
	Time   time.Time `json:"time"`
	Online int       `json:"online"`
}

type Stats struct {
	Active      int            `json:"active"`
	Rooms       map[string]int `json:"rooms"`
	Connections uint64         `json:"connections"`
	Messages    uint64         `json:"messages"`
	ReadPumps   int64          `json:"read_pumps"`
	WritePumps  int64          `json:"write_pumps"`
}

type Options struct {
	QueueSize  int
	MaxClients int
	MaxPerUser int
	PongWait   time.Duration
	PingPeriod time.Duration
	WriteWait  time.Duration
	Logger     *log.Logger
}

type joinRequest struct {
	c     *client
	reply chan error
}
type publication struct {
	c    *client
	text string
}
type sessionClose struct {
	id   [32]byte
	done chan struct{}
}

type Hub struct {
	options   Options
	join      chan joinRequest
	leave     chan *client
	publish   chan publication
	snapshots chan chan Stats
	sessions  chan sessionClose
	stop      chan struct{}
	done      chan struct{}
	once      sync.Once
	clientsWG sync.WaitGroup
	readers   atomic.Int64
	writers   atomic.Int64
}

// New starts exactly one owner goroutine. The caller must Close the hub.
func New(o Options) *Hub {
	if o.QueueSize <= 0 {
		o.QueueSize = 32
	}
	if o.MaxClients <= 0 {
		o.MaxClients = 256
	}
	if o.MaxPerUser <= 0 {
		o.MaxPerUser = 8
	}
	if o.PongWait <= 0 {
		o.PongWait = 45 * time.Second
	}
	if o.PingPeriod <= 0 || o.PingPeriod >= o.PongWait {
		o.PingPeriod = o.PongWait / 2
	}
	if o.WriteWait <= 0 {
		o.WriteWait = 5 * time.Second
	}
	if o.Logger == nil {
		o.Logger = log.Default()
	}
	h := &Hub{options: o, join: make(chan joinRequest), leave: make(chan *client), publish: make(chan publication, 64), snapshots: make(chan chan Stats), sessions: make(chan sessionClose), stop: make(chan struct{}), done: make(chan struct{})}
	go h.run()
	return h
}

// Close rejects new registrations, closes all transports, and joins both pumps.
func (h *Hub) Close() {
	h.once.Do(func() { close(h.stop) })
	<-h.done
	h.clientsWG.Wait()
}

func (h *Hub) Stats(ctx context.Context) (Stats, error) {
	reply := make(chan Stats, 1)
	select {
	case h.snapshots <- reply:
	case <-h.stop:
		return Stats{}, ErrClosed
	case <-ctx.Done():
		return Stats{}, ctx.Err()
	}
	select {
	case s := <-reply:
		return s, nil
	case <-ctx.Done():
		return Stats{}, ctx.Err()
	}
}

// DisconnectSession closes existing sockets for this login without logging JWTs.
func (h *Hub) DisconnectSession(ctx context.Context, raw string) error {
	r := sessionClose{SessionID(raw), make(chan struct{})}
	select {
	case h.sessions <- r:
	case <-h.stop:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hub) register(c *client) error {
	r := joinRequest{c, make(chan error, 1)}
	select {
	case h.join <- r:
		return <-r.reply
	case <-h.stop:
		return ErrClosed
	}
}

func (h *Hub) run() {
	defer close(h.done)
	clients := make(map[*client]struct{})
	var connections, sequence uint64
	count := func(room string) int {
		n := 0
		for c := range clients {
			if c.room == room {
				n++
			}
		}
		return n
	}
	remove := func(c *client, reason string) {
		if _, ok := clients[c]; !ok {
			return
		}
		delete(clients, c)
		close(c.done) // Only the owner closes done, and only for a registered client.
		c.closeTransport()
		h.options.Logger.Printf("chat disconnect user=%d room=%s reason=%s active=%d", c.identity.UserID, c.room, reason, len(clients))
	}
	// A full queue evicts only that client. No socket writes happen in this loop.
	broadcast := func(e Event) bool {
		removed := false
		for c := range clients {
			if c.room != e.Room {
				continue
			}
			select {
			case c.send <- e:
			default:
				remove(c, "slow_client")
				removed = true
			}
		}
		return removed
	}
	presence := func(room string) {
		for broadcast(Event{Type: "presence", Room: room, Time: time.Now().UTC(), Online: count(room)}) {
		}
	}
	for {
		select {
		case <-h.stop:
			for c := range clients {
				remove(c, "server_shutdown")
			}
			return
		case r := <-h.join:
			perUser := 0
			for c := range clients {
				if c.identity.UserID == r.c.identity.UserID {
					perUser++
				}
			}
			if len(clients) >= h.options.MaxClients || perUser >= h.options.MaxPerUser {
				r.reply <- errors.New("connection limit reached")
				continue
			}
			// All Adds happen in this loop before done closes, so Close cannot race Add.
			h.clientsWG.Add(1)
			clients[r.c] = struct{}{}
			connections++
			h.options.Logger.Printf("chat connect user=%d room=%s active=%d", r.c.identity.UserID, r.c.room, len(clients))
			r.reply <- nil
			presence(r.c.room)
		case c := <-h.leave:
			if _, ok := clients[c]; ok {
				remove(c, "transport_closed")
				presence(c.room)
			}
		case p := <-h.publish:
			if _, ok := clients[p.c]; !ok {
				continue
			}
			sequence++
			e := Event{Type: "message", Room: p.c.room, ID: sequence, Author: p.c.identity.Name, UserID: p.c.identity.UserID, Text: p.text, Time: time.Now().UTC(), Online: count(p.c.room)}
			if broadcast(e) {
				presence(p.c.room)
			}
		case r := <-h.sessions:
			affected := map[string]bool{}
			for c := range clients {
				if c.identity.Session == r.id {
					affected[c.room] = true
					remove(c, "logout")
				}
			}
			for room := range affected {
				presence(room)
			}
			close(r.done)
		case reply := <-h.snapshots:
			s := Stats{Active: len(clients), Rooms: map[string]int{}, Connections: connections, Messages: sequence, ReadPumps: h.readers.Load(), WritePumps: h.writers.Load()}
			for _, room := range Rooms() {
				s.Rooms[room.ID] = count(room.ID)
			}
			reply <- s
		}
	}
}
