package main

// The world: one engine, run on a fixed tick by one goroutine, fed by the
// sockets and feeding them back. Nothing here runs a rule; the rules are
// internal/lab's. This file is the order they run in and the messages that
// carry the result.

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andygeiss/wisp-engine"
	"github.com/andygeiss/wisp-engine/internal/lab"
	"github.com/andygeiss/wisp-engine/internal/wire"
	"github.com/andygeiss/wisp-engine/internal/ws"
)

// eventKind is what a socket can tell the world.
type eventKind uint8

const (
	evJoin eventKind = iota
	evLeave
	evIntent
)

// event is one thing a socket posted for the next tick to deal with.
type event struct {
	c      *client
	kind   eventKind
	intent wire.Intent
}

// world is the game every socket plays in. Everything but the inbox belongs
// to the goroutine running Tick.
type world struct {
	e          *wisp.Engine
	bouncers   *lab.Bouncers
	clients    []*client // the players, in the order they joined
	hub        hub
	logger     *slog.Logger
	maxPlayers int
	rate       int
	step       float64 // one tick, in milliseconds

	// tick counts ticks, and is read by the sockets' goroutines for a Pong.
	tick atomic.Uint32

	mu    sync.Mutex
	inbox []event

	actors []wire.Actor // one tick's snapshot, reused
	slow   int          // ticks that took longer than a step
}

// newWorld builds the world the configuration asks for: an engine with the
// lab's rules on it and the starting crowd in it.
func newWorld(cfg Config, logger *slog.Logger) *world {
	e := wisp.New(wisp.Config{FullscreenKey: wisp.KeyNone, MenuKey: wisp.KeyNone})
	lab.Setup(e)
	w := &world{
		e:          e,
		bouncers:   lab.NewBouncers(e, cfg.Bouncers),
		logger:     logger,
		maxPlayers: cfg.Players,
		rate:       cfg.Tick,
		step:       1000 / float64(cfg.Tick),
	}
	e.Simulate = w.bouncers.Simulate
	for range cfg.Crowd {
		w.bouncers.Add(rand.Float64()*lab.WorldW, rand.Float64()*lab.WorldH)
	}
	return w
}

// run ticks the world at its rate until ctx ends. It ticks once before the
// loop, because a ticker does not fire at zero; a tick that runs longer than
// a step is counted and, once a second, logged — the world runs slow rather
// than away, because a ticker drops the ticks it could not deliver.
func (w *world) run(ctx context.Context) {
	step := time.Second / time.Duration(w.rate)
	w.Tick()
	ticker := time.NewTicker(step)
	defer ticker.Stop()
	var loggedAt time.Time
	for {
		select {
		case <-ctx.Done():
			// Shutting down is not a fault; ctx.Err() is context.Canceled and
			// nobody needs to read that in a log.
			w.logger.Info("world stopped", "tick", w.tick.Load(), "slow_ticks", w.slow)
			return
		case <-ticker.C:
			start := time.Now()
			w.Tick()
			if took := time.Since(start); took > step {
				w.slow++
				if time.Since(loggedAt) > time.Second {
					loggedAt = time.Now()
					w.logger.Warn("tick ran long", "took", took, "step", step, "slow_ticks", w.slow)
				}
			}
		}
	}
}

// Tick runs the world forward by one step: the joins, the leaves and the
// intents that arrived; each player's steering and skills; the engine's own
// tick, which is the bounce and the movement; the rules that read the result;
// and one snapshot for everybody. It is a plain method so a test can call it
// and never wait on a clock.
func (w *world) Tick() {
	tick := w.tick.Add(1)
	for _, ev := range w.drain() {
		switch ev.kind {
		case evJoin:
			w.join(ev.c)
		case evLeave:
			w.leave(ev.c)
		case evIntent:
			w.intent(ev.c, ev.intent)
		}
	}

	for _, c := range w.clients {
		p := c.player
		w.e.Move(p.Entity, float64(c.dx), float64(c.dy))
		for s := range lab.NumSkills {
			if c.skills&(1<<s) == 0 {
				continue
			}
			affected, fired := p.Fire(s, w.bouncers)
			if !fired {
				continue
			}
			w.broadcast(wire.Append(nil, wire.Event{
				Tick: tick, Slot: uint16(p.Entity), Skill: uint8(s),
				X: float32(w.e.X[p.Entity]), Y: float32(w.e.Y[p.Entity]),
			}))
			for _, i := range affected {
				if s == lab.Strike {
					w.broadcast(wire.Append(nil, wire.Despawn{Slot: uint16(i)}))
				} else {
					w.broadcast(w.spawnOf(i))
				}
			}
		}
		c.skills = 0
	}

	w.e.Tick(w.step)
	for _, c := range w.clients {
		c.player.Tick(w.step)
	}

	// The snapshot ends a tick's batch: a client applies messages up to and
	// including one, so everything the tick has to say — including the
	// player's own You — goes ahead of it.
	snapshot := w.snapshot(tick)
	for _, c := range w.clients {
		c.send(wire.Append(nil, wire.You{Tick: tick, Cooldowns: c.cooldowns()}), snapshot)
	}
}

// post hands the next tick something a socket learned. Any goroutine may
// call it.
func (w *world) post(ev event) {
	w.mu.Lock()
	w.inbox = append(w.inbox, ev)
	w.mu.Unlock()
}

// drain takes everything posted since the last tick, in the order it came.
func (w *world) drain() []event {
	w.mu.Lock()
	defer w.mu.Unlock()
	events := w.inbox
	w.inbox = nil
	return events
}

// join gives a socket a player: an entity, a Welcome naming it, a Spawn for
// everything that exists, and a Spawn of the newcomer to everybody else. A
// world that is full says so and closes the socket.
func (w *world) join(c *client) {
	if len(w.clients) >= w.maxPlayers {
		c.stop(ws.ClosePolicy, "the game is full")
		return
	}
	p := lab.NewPlayer(w.e)
	c.player = p
	w.clients = append(w.clients, c)

	// The whole world, in one batch: a thousand spawns one message at a time
	// would fill the queue a slow client is judged by.
	msgs := []([]byte){wire.Append(nil, wire.Welcome{
		Version: wire.Version, TickRate: uint8(w.rate),
		WorldW: uint16(lab.WorldW), WorldH: uint16(lab.WorldH),
		You: uint16(p.Entity), Tick: w.tick.Load(),
	})}
	for i := range w.e.Slots() {
		if w.e.Live(i) {
			msgs = append(msgs, w.spawnOf(i))
		}
	}
	c.send(msgs...)

	spawn := w.spawnOf(p.Entity)
	for _, o := range w.clients {
		if o != c {
			o.send(spawn)
		}
	}
	w.logger.Info("player joined", "slot", p.Entity, "players", len(w.clients))
}

// leave takes a socket's player out of the world and tells everybody.
func (w *world) leave(c *client) {
	n := slices.Index(w.clients, c)
	if n < 0 {
		return
	}
	w.clients = slices.Delete(w.clients, n, n+1)
	slot := c.player.Entity
	w.e.Delete(slot)
	w.broadcast(wire.Append(nil, wire.Despawn{Slot: uint16(slot)}))
	w.logger.Info("player left", "slot", slot, "players", len(w.clients))
}

// intent records what a player is trying to do. The axis is reduced to its
// sign and held until the next intent says otherwise; a press is kept until
// the tick has looked at it, so a tap between two ticks is not lost; a skill
// bit the game does not know is dropped.
func (w *world) intent(c *client, m wire.Intent) {
	if !slices.Contains(w.clients, c) {
		return
	}
	c.dx, c.dy = sign(m.DX), sign(m.DY)
	c.skills |= m.Skills & (1<<lab.NumSkills - 1)
}

// broadcast sends one message to every player.
func (w *world) broadcast(b []byte) {
	for _, c := range w.clients {
		c.send(b)
	}
}

// spawnOf describes entity i the way a client needs to add it.
func (w *world) spawnOf(i int) []byte {
	e := w.e
	return wire.Append(nil, wire.Spawn{
		Slot: uint16(i), Image: uint8(e.ImageIndex[i]), Column: uint8(e.ImageColumn[i]),
		Row: uint8(e.ImageRow[i]), Width: uint16(e.SpriteWidth[i]), Height: uint16(e.SpriteHeight[i]),
		X: float32(e.X[i]), Y: float32(e.Y[i]), Z: int8(e.Z[i]),
		Alpha: uint8(e.Alpha[i]*255 + 0.5), State: e.State[i],
	})
}

// snapshot encodes every actor as of this tick. It is a fresh slice every
// tick because every player's writer reads it after this tick has moved on.
func (w *world) snapshot(tick uint32) []byte {
	e := w.e
	w.actors = w.actors[:0]
	for i := range e.Slots() {
		if !e.Live(i) {
			continue
		}
		w.actors = append(w.actors, wire.Actor{
			Slot: uint16(i), X: float32(e.X[i]), Y: float32(e.Y[i]),
			State: e.State[i], Row: uint8(e.ImageRow[i]),
		})
	}
	return wire.Append(nil, wire.Snapshot{Tick: tick, Actors: w.actors})
}

// sign reduces an axis to -1, 0 or 1, so a hand-crafted 100 pushes as hard
// as a 1.
func sign(v int8) int8 {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 0
}
