package main

// One socket: the goroutine that reads it, the goroutine that writes it, and
// the hub that counts both so shutdown can wait for them.

import (
	"net/http"
	"sync"
	"time"

	"github.com/andygeiss/wisp-engine/internal/lab"
	"github.com/andygeiss/wisp-engine/internal/wire"
	"github.com/andygeiss/wisp-engine/internal/ws"
)

const (
	// pingInterval is how often a quiet socket is pinged. The read deadline
	// is twice it, so a peer that has gone is found within two pings.
	pingInterval = 5 * time.Second
	// writeTimeout bounds one write to a client.
	writeTimeout = 5 * time.Second
	// maxClientMessage is the largest frame a client may send. An intent is
	// six bytes; anything near this is not a client.
	maxClientMessage = 1024
	// maxMessagesPerSecond is the most a client may send before it is closed:
	// one every frame on a 120 Hz display, which no human reaches.
	maxMessagesPerSecond = 120
	// outboundQueue is how many ticks a client may fall behind before it is
	// closed, so a slow client cannot slow the world.
	outboundQueue = 64
)

// client is one socket. The world's tick goroutine owns player, the axis and
// the presses; everything else is shared and says how.
//
// Only the writing goroutine ever closes the socket. A close frame is a write,
// and a write to a client that has stopped reading blocks until its deadline,
// which is time the world's tick must never spend: stop closes a channel, and
// the writer does the closing when it wakes.
type client struct {
	conn *ws.Conn
	w    *world
	out  chan [][]byte // batches to write, in order; full means too slow
	done chan struct{} // closed once, by stop
	once sync.Once
	// code and reason are set by stop before done closes, and read by the
	// writer after, so the channel orders them.
	code   uint16
	reason string

	player *lab.Player
	dx, dy int8
	skills uint8
}

func newClient(conn *ws.Conn, w *world) *client {
	return &client{conn: conn, w: w, out: make(chan [][]byte, outboundQueue), done: make(chan struct{})}
}

// serveWS is GET /ws: the handshake, then this goroutine reads the socket
// until it closes while another writes it.
func (w *world) serveWS(rw http.ResponseWriter, r *http.Request) {
	conn, err := ws.Accept(rw, r)
	if err != nil {
		w.logger.Debug("socket refused", "error", err)
		return
	}
	conn.MaxMessage = maxClientMessage
	conn.ReadTimeout = 2 * pingInterval
	conn.WriteTimeout = writeTimeout

	c := newClient(conn, w)
	if !w.hub.add(c) {
		// No writer was started for it, so the socket is closed here.
		conn.Close(ws.CloseGoingAway, "server shutting down")
		return
	}
	defer w.hub.remove(c)
	w.hub.wg.Go(c.write)
	c.read()
}

// read is the socket's reading half. The first message has to be a Hello
// with this version; after it, intents go to the world and pings are
// answered here, with the tick as it is now. It returns when the socket is
// done, and posts the leave if a join was posted.
func (c *client) read() {
	joined := false
	defer func() {
		c.stop(ws.CloseNormal, "")
		if joined {
			c.w.post(event{c: c, kind: evLeave})
		}
	}()
	window, n := time.Now(), 0
	for {
		b, err := c.conn.Read()
		if err != nil {
			return
		}
		if now := time.Now(); now.Sub(window) >= time.Second {
			window, n = now, 0
		}
		if n++; n > maxMessagesPerSecond {
			c.stop(ws.ClosePolicy, "more than 120 messages a second")
			return
		}
		m, err := wire.Decode(b)
		if err != nil {
			c.stop(ws.ClosePolicy, err.Error())
			return
		}
		switch m := m.(type) {
		case wire.Hello:
			if joined || m.Version != wire.Version {
				c.stop(ws.ClosePolicy, "hello: want protocol version 1, once")
				return
			}
			joined = true
			c.w.post(event{c: c, kind: evJoin})
		case wire.Intent:
			if !joined {
				c.stop(ws.ClosePolicy, "an intent before hello")
				return
			}
			c.w.post(event{c: c, kind: evIntent, intent: m})
		case wire.Ping:
			c.send(wire.Append(nil, wire.Pong{T: m.T, Tick: c.w.tick.Load()}))
		default:
			c.stop(ws.ClosePolicy, "that message is the server's to send")
			return
		}
	}
}

// write is the socket's writing half: batches in order, and a ping when
// nothing has been said for a while. It ends when the client is stopped or a
// write fails, and it is the goroutine that closes the socket, with the code
// stop was given. A write to a peer that has stopped reading ends at the
// write deadline, so a stopped client is closed within one deadline of the
// stop and the world's tick waits for none of it.
func (c *client) write() {
	defer func() { c.conn.Close(c.code, c.reason) }()
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()
	for {
		select {
		case <-c.done:
			return
		case msgs := <-c.out:
			for _, b := range msgs {
				if err := c.conn.Write(b); err != nil {
					c.stop(ws.CloseNormal, "")
					return
				}
			}
		case <-ping.C:
			if err := c.conn.Ping(nil); err != nil {
				c.stop(ws.CloseNormal, "")
				return
			}
		}
	}
}

// send queues one batch for the writer without ever waiting for it. A client
// whose queue is full is closed: the world does not slow down for anybody.
func (c *client) send(msgs ...[]byte) {
	if c.stopped() {
		return
	}
	select {
	case c.out <- msgs:
	default:
		c.stop(ws.ClosePolicy, "too slow to keep up")
	}
}

// stop asks the writer to close the socket with code and reason, once; later
// calls do nothing. It never blocks, so the world's tick may call it.
func (c *client) stop(code uint16, reason string) {
	c.once.Do(func() {
		c.code, c.reason = code, reason
		close(c.done)
	})
}

func (c *client) stopped() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// cooldowns is the player's cooldowns as the You message carries them.
func (c *client) cooldowns() []uint16 {
	cd := make([]uint16, lab.NumSkills)
	for s, ms := range c.player.Cooldown {
		cd[s] = uint16(min(ms+0.5, 65535))
	}
	return cd
}

// hub is every socket that is open, and the wait for them to end. Shutdown
// never drains a hijacked connection, so the sockets are work the requests
// started and did not wait for: closeAll tells them to go and Wait waits.
type hub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
	closing bool
	wg      sync.WaitGroup
}

// add registers a socket and counts its reader, or refuses it once the hub
// is closing.
func (h *hub) add(c *client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closing {
		return false
	}
	if h.clients == nil {
		h.clients = make(map[*client]struct{})
	}
	h.clients[c] = struct{}{}
	h.wg.Add(1)
	return true
}

// remove forgets a socket whose reader has ended.
func (h *hub) remove(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	h.wg.Done()
}

// closeAll tells every socket the server is going away and refuses any that
// arrive after. http.Server.RegisterOnShutdown calls it.
func (h *hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closing = true
	for c := range h.clients {
		c.stop(ws.CloseGoingAway, "server shutting down")
	}
}

// Wait returns once every socket's reader and writer have ended.
func (h *hub) Wait() { h.wg.Wait() }
