// Package wire is the lab's network protocol: the messages a client and the
// server exchange, and their bytes.
//
// Version 1. Big-endian, one WebSocket binary message per wire message, and
// the first byte says which. It is written by hand rather than with
// encoding/json for the reason the engine's sheet scanner is: the client is
// compiled by TinyGo, which pays for reflection in kilobytes and fails at it
// in a browser. [Append] puts a message on the end of a byte slice and
// [Decode] reads one back. Nothing here touches a socket, so it tests on the
// host.
//
// Whatever Decode accepts is safe to hand to the engine: a position is always
// a finite number, a count always matches the bytes that follow it, and a
// message is exactly as long as its kind says. Anything else is one of the
// four errors, never a panic.
//
// A server tick's messages end with its [Snapshot]. Everything the tick has
// to say — a Spawn, a Despawn, an Event, the player's own You — is sent ahead
// of it, so a client that applies messages up to and including the next
// snapshot has applied the whole tick.
package wire

import (
	"encoding/binary"
	"errors"
	"math"
)

// Version is the protocol version. A [Hello] or a [Welcome] carrying another
// one is a peer this package cannot talk to.
const Version = 1

// Kind is the first byte of every message.
type Kind uint8

// The message kinds. Zero is left out so an empty buffer never decodes as
// anything.
const (
	KindHello Kind = iota + 1
	KindIntent
	KindPing
	KindWelcome
	KindSpawn
	KindDespawn
	KindSnapshot
	KindYou
	KindEvent
	KindPong
)

var (
	// ErrKind means the first byte named no message.
	ErrKind = errors.New("wire: unknown message kind")
	// ErrShort means the bytes ended before the message did.
	ErrShort = errors.New("wire: message too short")
	// ErrLong means the bytes went on after the message ended.
	ErrLong = errors.New("wire: bytes after the message")
	// ErrValue means a field held something the engine could not apply: a
	// position that is not a finite number.
	ErrValue = errors.New("wire: value out of range")
)

// Message is one of the ten message types. Only this package's types are
// messages, because only they know their bytes.
type Message interface {
	// Kind is the message's first byte.
	Kind() Kind
	appendTo(b []byte) []byte
}

// Hello is the client's first message: the protocol version it speaks.
type Hello struct {
	Version uint8
}

// Intent is what the player is trying to do, and the only thing a client
// sends about the world. DX and DY are an axis, each -1, 0 or 1 — the server
// reduces anything else to its sign. Bit i of Skills says skill i was pressed
// since the last intent. Seq counts intents, so a log can tell them apart.
type Intent struct {
	Seq    uint16
	DX, DY int8
	Skills uint8
}

// Ping carries the client's own clock, in milliseconds, so the [Pong] that
// echoes it measures the round trip.
type Ping struct {
	T uint32
}

// Welcome is the server's first message. TickRate is how many times a second
// the world runs, which the client's draw needs to blend between two ticks.
// WorldW and WorldH are in pixels. You is the server slot of the player's own
// entity, and Tick the server's tick when it was sent.
type Welcome struct {
	Version  uint8
	TickRate uint8
	WorldW   uint16
	WorldH   uint16
	You      uint16
	Tick     uint32
}

// Spawn is an entity arriving: everything [wisp.Sprite] needs, keyed by the
// server's slot. Alpha is opacity in 255ths.
type Spawn struct {
	Slot   uint16
	Image  uint8
	Column uint8
	Row    uint8
	Width  uint16
	Height uint16
	X, Y   float32
	Z      int8
	Alpha  uint8
	State  uint64
}

// Despawn is an entity leaving.
type Despawn struct {
	Slot uint16
}

// Actor is one entity's share of a [Snapshot]: where it is, its state bits and
// the sheet row it is on.
type Actor struct {
	Slot  uint16
	X, Y  float32
	State uint64
	Row   uint8
}

// Snapshot is the whole world as of one server tick: every actor, every tick,
// on purpose. Sending only what changed is a follow-up that waits for a
// number.
type Snapshot struct {
	Tick   uint32
	Actors []Actor
}

// You is what the server knows about this player alone: milliseconds of
// cooldown left on each skill, in skill order.
type You struct {
	Tick      uint32
	Cooldowns []uint16
}

// Event says a skill fired: whose, which, and where.
type Event struct {
	Tick  uint32
	Slot  uint16
	Skill uint8
	X, Y  float32
}

// Pong echoes a [Ping]'s clock and adds the server's tick.
type Pong struct {
	T    uint32
	Tick uint32
}

// actorBytes is one Actor on the wire: slot, x, y, state, row.
const actorBytes = 2 + 4 + 4 + 8 + 1

// Append encodes m onto the end of b and returns the longer slice. It
// allocates only when b has no room, so a server encoding one snapshot a tick
// reuses one buffer.
func Append(b []byte, m Message) []byte {
	return m.appendTo(append(b, byte(m.Kind())))
}

// Decode reads the one message b holds. It returns [ErrKind], [ErrShort],
// [ErrLong] or [ErrValue] rather than a message that is not all there.
func Decode(b []byte) (Message, error) {
	if len(b) == 0 {
		return nil, ErrShort
	}
	r := reader{b: b[1:]}
	var m Message
	switch Kind(b[0]) {
	case KindHello:
		m = Hello{Version: r.u8()}
	case KindIntent:
		m = Intent{Seq: r.u16(), DX: r.i8(), DY: r.i8(), Skills: r.u8()}
	case KindPing:
		m = Ping{T: r.u32()}
	case KindWelcome:
		m = Welcome{Version: r.u8(), TickRate: r.u8(), WorldW: r.u16(), WorldH: r.u16(), You: r.u16(), Tick: r.u32()}
	case KindSpawn:
		m = Spawn{
			Slot: r.u16(), Image: r.u8(), Column: r.u8(), Row: r.u8(),
			Width: r.u16(), Height: r.u16(), X: r.f32(), Y: r.f32(),
			Z: r.i8(), Alpha: r.u8(), State: r.u64(),
		}
	case KindDespawn:
		m = Despawn{Slot: r.u16()}
	case KindSnapshot:
		m = r.snapshot()
	case KindYou:
		m = r.you()
	case KindEvent:
		m = Event{Tick: r.u32(), Slot: r.u16(), Skill: r.u8(), X: r.f32(), Y: r.f32()}
	case KindPong:
		m = Pong{T: r.u32(), Tick: r.u32()}
	default:
		return nil, ErrKind
	}
	if err := r.done(); err != nil {
		return nil, err
	}
	return m, nil
}

// Kind returns [KindHello].
func (Hello) Kind() Kind { return KindHello }

// Kind returns [KindIntent].
func (Intent) Kind() Kind { return KindIntent }

// Kind returns [KindPing].
func (Ping) Kind() Kind { return KindPing }

// Kind returns [KindWelcome].
func (Welcome) Kind() Kind { return KindWelcome }

// Kind returns [KindSpawn].
func (Spawn) Kind() Kind { return KindSpawn }

// Kind returns [KindDespawn].
func (Despawn) Kind() Kind { return KindDespawn }

// Kind returns [KindSnapshot].
func (Snapshot) Kind() Kind { return KindSnapshot }

// Kind returns [KindYou].
func (You) Kind() Kind { return KindYou }

// Kind returns [KindEvent].
func (Event) Kind() Kind { return KindEvent }

// Kind returns [KindPong].
func (Pong) Kind() Kind { return KindPong }

func (m Hello) appendTo(b []byte) []byte { return append(b, m.Version) }

func (m Intent) appendTo(b []byte) []byte {
	b = binary.BigEndian.AppendUint16(b, m.Seq)
	return append(b, byte(m.DX), byte(m.DY), m.Skills)
}

func (m Ping) appendTo(b []byte) []byte { return binary.BigEndian.AppendUint32(b, m.T) }

func (m Welcome) appendTo(b []byte) []byte {
	b = append(b, m.Version, m.TickRate)
	b = binary.BigEndian.AppendUint16(b, m.WorldW)
	b = binary.BigEndian.AppendUint16(b, m.WorldH)
	b = binary.BigEndian.AppendUint16(b, m.You)
	return binary.BigEndian.AppendUint32(b, m.Tick)
}

func (m Spawn) appendTo(b []byte) []byte {
	b = binary.BigEndian.AppendUint16(b, m.Slot)
	b = append(b, m.Image, m.Column, m.Row)
	b = binary.BigEndian.AppendUint16(b, m.Width)
	b = binary.BigEndian.AppendUint16(b, m.Height)
	b = appendF32(b, m.X)
	b = appendF32(b, m.Y)
	b = append(b, byte(m.Z), m.Alpha)
	return binary.BigEndian.AppendUint64(b, m.State)
}

func (m Despawn) appendTo(b []byte) []byte { return binary.BigEndian.AppendUint16(b, m.Slot) }

func (m Snapshot) appendTo(b []byte) []byte {
	b = binary.BigEndian.AppendUint32(b, m.Tick)
	b = binary.BigEndian.AppendUint16(b, uint16(len(m.Actors)))
	for _, a := range m.Actors {
		b = binary.BigEndian.AppendUint16(b, a.Slot)
		b = appendF32(b, a.X)
		b = appendF32(b, a.Y)
		b = binary.BigEndian.AppendUint64(b, a.State)
		b = append(b, a.Row)
	}
	return b
}

func (m You) appendTo(b []byte) []byte {
	b = binary.BigEndian.AppendUint32(b, m.Tick)
	b = append(b, uint8(len(m.Cooldowns)))
	for _, c := range m.Cooldowns {
		b = binary.BigEndian.AppendUint16(b, c)
	}
	return b
}

func (m Event) appendTo(b []byte) []byte {
	b = binary.BigEndian.AppendUint32(b, m.Tick)
	b = binary.BigEndian.AppendUint16(b, m.Slot)
	b = append(b, m.Skill)
	b = appendF32(b, m.X)
	return appendF32(b, m.Y)
}

func (m Pong) appendTo(b []byte) []byte {
	b = binary.BigEndian.AppendUint32(b, m.T)
	return binary.BigEndian.AppendUint32(b, m.Tick)
}

func appendF32(b []byte, v float32) []byte {
	return binary.BigEndian.AppendUint32(b, math.Float32bits(v))
}

// reader walks a message body. The first thing that goes wrong is kept and
// every later read returns zero, so a decoder is a straight line of field
// reads with one check at the end.
type reader struct {
	b   []byte
	err error
}

// take returns the next n bytes, or nil once the body has run out.
func (r *reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if len(r.b) < n {
		r.err = ErrShort
		return nil
	}
	v := r.b[:n]
	r.b = r.b[n:]
	return v
}

func (r *reader) u8() uint8 {
	if v := r.take(1); v != nil {
		return v[0]
	}
	return 0
}

func (r *reader) i8() int8 { return int8(r.u8()) }

func (r *reader) u16() uint16 {
	if v := r.take(2); v != nil {
		return binary.BigEndian.Uint16(v)
	}
	return 0
}

func (r *reader) u32() uint32 {
	if v := r.take(4); v != nil {
		return binary.BigEndian.Uint32(v)
	}
	return 0
}

func (r *reader) u64() uint64 {
	if v := r.take(8); v != nil {
		return binary.BigEndian.Uint64(v)
	}
	return 0
}

// f32 reads a position. NaN and the infinities are refused: the engine would
// sort, cull and draw with them, and nothing downstream checks again.
func (r *reader) f32() float32 {
	f := math.Float32frombits(r.u32())
	if r.err == nil && (f != f || f > math.MaxFloat32 || f < -math.MaxFloat32) {
		r.err = ErrValue
	}
	return f
}

// snapshot reads the count, then checks the count against the bytes before
// allocating for it. A count that lies is an error, not an allocation.
func (r *reader) snapshot() Snapshot {
	s := Snapshot{Tick: r.u32()}
	n := int(r.u16())
	if r.err != nil {
		return s
	}
	if len(r.b) < n*actorBytes {
		r.err = ErrShort
		return s
	}
	s.Actors = make([]Actor, n)
	for i := range s.Actors {
		s.Actors[i] = Actor{Slot: r.u16(), X: r.f32(), Y: r.f32(), State: r.u64(), Row: r.u8()}
	}
	return s
}

func (r *reader) you() You {
	y := You{Tick: r.u32()}
	n := int(r.u8())
	if r.err != nil {
		return y
	}
	if len(r.b) < n*2 {
		r.err = ErrShort
		return y
	}
	y.Cooldowns = make([]uint16, n)
	for i := range y.Cooldowns {
		y.Cooldowns[i] = r.u16()
	}
	return y
}

// done is the one check at the end: the first error, or bytes left over.
func (r *reader) done() error {
	if r.err != nil {
		return r.err
	}
	if len(r.b) != 0 {
		return ErrLong
	}
	return nil
}
