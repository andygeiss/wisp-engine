// Package replica keeps a copy of the server's world in an engine: the
// server's slots mapped to the engine's indices, the server's messages in the
// order they arrived, and the depth rule that decides how many of them one
// tick applies.
//
// It is pure Go — no syscall/js, no socket — so it tests on the host and
// compiles into the browser module unchanged. A client feeds it every
// message with [Replica.Push] as the message arrives and hands
// [Replica.Tick] to the engine as its Simulate, so a snapshot is applied on
// the engine's own tick: Tick has just copied PrevX from X, so the previous
// snapshot and this one are exactly the pair [wisp.Engine.DrawPos] blends,
// and the client draws one server tick behind. That is the whole reason the
// blend was built before this.
package replica

import (
	"errors"

	"github.com/andygeiss/wisp-engine"
	"github.com/andygeiss/wisp-engine/internal/wire"
)

// The depth rule. The queue is measured in snapshots, because a snapshot is
// what a tick spends, and the depth a client likes is one: a tick then always
// has a snapshot to apply and never has to guess.
const (
	// fastForwardAbove is the depth past which a tick applies two snapshots,
	// closing the gap in a few ticks rather than in one pop.
	fastForwardAbove = 2
	// drainAfter is how many ticks in a row may find fastForwardAbove
	// snapshots before one of them applies two. A fast-forward stops at that
	// depth and a slow clock drifts up to it, and either leaves the client a
	// tick further behind for good; an early snapshot puts the queue there
	// for one tick and the late one takes it back. Thirty ticks is a second
	// at the 30 Hz the wire runs, which no jitter lasts.
	drainAfter = 30
	// catchUpAbove is the depth of a tab that slept: everything is applied at
	// once, because a minute of fast-forward is worse than one jump.
	catchUpAbove = 64
)

// ErrVersion means the server's Welcome named a protocol this package does
// not speak.
var ErrVersion = errors.New("replica: the server speaks another protocol version")

// Replica is the server's world, kept in E.
type Replica struct {
	// E is the engine the world is kept in. Every actor the server sends is
	// an entity in it, with [wisp.StateRemote] set so the engine never moves
	// it on its own.
	E *wisp.Engine

	// From the Welcome, once Welcomed is true: the server's tick rate, the
	// world's size, and which slot is this player.
	Welcomed bool
	TickRate uint8
	WorldW   uint16
	WorldH   uint16
	YouSlot  uint16

	// Cooldowns is the last You: milliseconds left on each skill, in skill
	// order. Nil until the first one.
	Cooldowns []uint16

	// OnEvent is called for every Event as it is applied, in order, so a
	// client can play the feel of a skill when the server says it fired.
	OnEvent func(wire.Event)

	// Err is set when a message could not be applied at all — a Welcome
	// from another protocol version — and stays set.
	Err error

	// The numbers the lab prints: how often a tick had nothing to apply,
	// applied two, took a lasting surplus back, or applied everything; how
	// many snapshots it has applied; and the server tick of the last one.
	Held         int
	FastForwards int
	Drains       int
	CatchUps     int
	Applied      int
	LastTick     uint32

	// LastDepth is how many snapshots the last tick found waiting, before it
	// applied any: the number the rule judged. A HUD prints this one rather
	// than [Replica.Depth], which fills between the frames of one tick and
	// so flips with where the snapshot landed.
	LastDepth int

	queue     []wire.Message
	head      int
	snapshots int
	surplus   int // ticks in a row that found fastForwardAbove snapshots
	slots     map[uint16]int
}

// New returns an empty replica on e.
func New(e *wisp.Engine) *Replica {
	return &Replica{E: e, slots: make(map[uint16]int)}
}

// Push queues one message from the server, in arrival order. A Pong is not
// the world's business and is dropped; the client times it as it arrives.
func (r *Replica) Push(m wire.Message) {
	switch m.(type) {
	case wire.Pong, wire.Hello, wire.Intent, wire.Ping:
		return
	case wire.Snapshot:
		r.snapshots++
	}
	r.queue = append(r.queue, m)
}

// Depth is how many snapshots are waiting to be applied.
func (r *Replica) Depth() int { return r.snapshots }

// Pending is how many messages of any kind are waiting.
func (r *Replica) Pending() int { return len(r.queue) - r.head }

// Index returns the engine index of the entity in server slot slot, or -1
// when no such entity has been spawned.
func (r *Replica) Index(slot uint16) int {
	if i, ok := r.slots[slot]; ok {
		return i
	}
	return -1
}

// You returns the engine index of this player's own entity, or -1 until the
// Welcome and its Spawn have both been applied.
func (r *Replica) You() int {
	if !r.Welcomed {
		return -1
	}
	return r.Index(r.YouSlot)
}

// Tick applies the server's messages by the depth rule: up to and including
// one snapshot in the usual case, two when the queue has run ahead or has
// held one more than it needs for a second, all of them when a tab slept,
// and nothing at all when the queue is empty — a freeze, not a guess. Hand
// it to the engine as Simulate.
func (r *Replica) Tick() {
	r.LastDepth = r.snapshots
	if r.snapshots == fastForwardAbove {
		r.surplus++
	} else {
		r.surplus = 0
	}
	frames := 1
	switch {
	case r.snapshots == 0:
		if r.Applied > 0 {
			r.Held++
		}
		return
	case r.snapshots > catchUpAbove:
		frames = r.snapshots
		r.CatchUps++
	case r.snapshots > fastForwardAbove:
		frames = 2
		r.FastForwards++
	case r.surplus >= drainAfter:
		frames = 2
		r.surplus = 0
		r.Drains++
	}
	for range frames {
		r.frame()
	}
}

// frame applies messages up to and including the next snapshot.
func (r *Replica) frame() {
	for r.head < len(r.queue) {
		m := r.queue[r.head]
		r.queue[r.head] = nil
		r.head++
		if r.head == len(r.queue) {
			r.queue, r.head = r.queue[:0], 0
		}
		r.apply(m)
		if _, ok := m.(wire.Snapshot); ok {
			r.snapshots--
			return
		}
	}
}

// apply puts one message into the engine.
func (r *Replica) apply(m wire.Message) {
	e := r.E
	switch m := m.(type) {
	case wire.Welcome:
		if m.Version != wire.Version {
			r.Err = ErrVersion
			return
		}
		r.Welcomed = true
		r.TickRate, r.WorldW, r.WorldH, r.YouSlot = m.TickRate, m.WorldW, m.WorldH, m.You
	case wire.Spawn:
		// A slot the server reuses is a slot this side has to let go of
		// first, or two entities would answer to one name.
		if i, ok := r.slots[m.Slot]; ok {
			e.Delete(i)
		}
		r.slots[m.Slot] = e.Add(wisp.Sprite{
			Alpha: float64(m.Alpha) / 255, Column: int(m.Column), Height: float64(m.Height),
			Image: int(m.Image), Row: int(m.Row), State: m.State | wisp.StateRemote,
			Width: float64(m.Width), X: float64(m.X), Y: float64(m.Y), Z: int(m.Z),
		})
	case wire.Despawn:
		if i, ok := r.slots[m.Slot]; ok {
			e.Delete(i)
			delete(r.slots, m.Slot)
		}
	case wire.Snapshot:
		for _, a := range m.Actors {
			i, ok := r.slots[a.Slot]
			if !ok {
				continue
			}
			// Written, not placed: Tick has already kept the previous
			// position, and this one is what it blends toward.
			e.X[i], e.Y[i] = float64(a.X), float64(a.Y)
			e.State[i] = a.State | wisp.StateRemote
			if row := int(a.Row); e.ImageRow[i] != row {
				// The frame starts over when the animation changes, the way
				// Play starts one over; a row that is the same keeps its
				// frame, so the animation is the client's to time.
				e.ImageRow[i] = row
				e.FrameOffset[i] = 0
				e.FrameTime[i] = 0
			}
		}
		r.LastTick = m.Tick
		r.Applied++
	case wire.You:
		r.Cooldowns = m.Cooldowns
	case wire.Event:
		if r.OnEvent != nil {
			r.OnEvent(m)
		}
	}
}
