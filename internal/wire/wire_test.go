package wire_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/andygeiss/wisp-engine/internal/wire"
)

// every is one of each message, with every field set to something that is not
// its zero value, so a field the encoder forgot shows up in a comparison.
func every() []wire.Message {
	return []wire.Message{
		wire.Hello{Version: 1},
		wire.Intent{Seq: 513, DX: -1, DY: 1, Skills: 0b101},
		wire.Ping{T: 123456789},
		wire.Welcome{Version: 1, TickRate: 30, WorldW: 1280, WorldH: 768, You: 7, Tick: 99},
		wire.Spawn{
			Slot: 12, Image: 1, Column: 2, Row: 3, Width: 32, Height: 32,
			X: 640.5, Y: -1.25, Z: -1, Alpha: 153, State: 1<<63 | 0x2AAA,
		},
		wire.Despawn{Slot: 65535},
		wire.Snapshot{Tick: 4000000000, Actors: []wire.Actor{
			{Slot: 1, X: 10, Y: 20, State: 0x3041, Row: 2},
			{Slot: 900, X: 1279.9, Y: 767.9, State: 0, Row: 11},
		}},
		wire.You{Tick: 5, Cooldowns: []uint16{1000, 0, 4990}},
		wire.Event{Tick: 6, Slot: 3, Skill: 2, X: 100, Y: 200},
		wire.Pong{T: 123456789, Tick: 7},
	}
}

func TestEveryMessageRoundTrips(t *testing.T) {
	t.Parallel()
	for _, m := range every() {
		b := wire.Append(nil, m)
		if b[0] != byte(m.Kind()) {
			t.Errorf("%T: first byte is %d, want its kind %d", m, b[0], m.Kind())
		}
		got, err := wire.Decode(b)
		if err != nil {
			t.Errorf("%T: Decode: %v", m, err)
			continue
		}
		if !reflect.DeepEqual(got, m) {
			t.Errorf("%T round-tripped to %+v, want %+v", m, got, m)
		}
	}
}

// An empty Snapshot and an empty You have a count and nothing after it, which
// is the shape the count check has to get right in both directions.
func TestEmptyListsRoundTrip(t *testing.T) {
	t.Parallel()
	for _, m := range []wire.Message{wire.Snapshot{Tick: 1}, wire.You{Tick: 2}} {
		got, err := wire.Decode(wire.Append(nil, m))
		if err != nil {
			t.Fatalf("%T: %v", m, err)
		}
		switch g := got.(type) {
		case wire.Snapshot:
			if g.Tick != 1 || len(g.Actors) != 0 {
				t.Errorf("Snapshot = %+v", g)
			}
		case wire.You:
			if g.Tick != 2 || len(g.Cooldowns) != 0 {
				t.Errorf("You = %+v", g)
			}
		}
	}
}

func TestAppendKeepsWhatWasThere(t *testing.T) {
	t.Parallel()
	b := wire.Append([]byte("prefix"), wire.Ping{T: 1})
	if string(b[:6]) != "prefix" {
		t.Errorf("Append overwrote the buffer: %q", b)
	}
	if _, err := wire.Decode(b[6:]); err != nil {
		t.Errorf("the appended message does not decode: %v", err)
	}
}

// Every prefix of every message is short, and a truncated body is an error
// rather than a panic or a message with zeros in it.
func TestDecodeRefusesAShortMessage(t *testing.T) {
	t.Parallel()
	for _, m := range every() {
		b := wire.Append(nil, m)
		for n := 1; n < len(b); n++ {
			if _, err := wire.Decode(b[:n]); !errors.Is(err, wire.ErrShort) {
				t.Errorf("%T cut to %d of %d bytes: err = %v, want ErrShort", m, n, len(b), err)
			}
		}
	}
	if _, err := wire.Decode(nil); !errors.Is(err, wire.ErrShort) {
		t.Errorf("empty input: err = %v, want ErrShort", err)
	}
}

func TestDecodeRefusesALongMessage(t *testing.T) {
	t.Parallel()
	for _, m := range every() {
		b := append(wire.Append(nil, m), 0)
		if _, err := wire.Decode(b); !errors.Is(err, wire.ErrLong) {
			t.Errorf("%T with a byte after it: err = %v, want ErrLong", m, err)
		}
	}
}

func TestDecodeRefusesAnUnknownKind(t *testing.T) {
	t.Parallel()
	for _, k := range []byte{0, 11, 255} {
		if _, err := wire.Decode([]byte{k, 0, 0, 0, 0}); !errors.Is(err, wire.ErrKind) {
			t.Errorf("kind %d: err = %v, want ErrKind", k, err)
		}
	}
}

// A position that is not a finite number would be sorted, culled and drawn
// with, and nothing after the decoder checks again.
func TestDecodeRefusesAPositionThatIsNotANumber(t *testing.T) {
	t.Parallel()
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	for _, m := range []wire.Message{
		wire.Spawn{X: nan},
		wire.Spawn{Y: inf},
		wire.Event{X: float32(math.Inf(-1))},
		wire.Snapshot{Actors: []wire.Actor{{}, {Y: nan}}},
	} {
		if _, err := wire.Decode(wire.Append(nil, m)); !errors.Is(err, wire.ErrValue) {
			t.Errorf("%+v: err = %v, want ErrValue", m, err)
		}
	}
}

// A count that claims more actors than the bytes hold is an error before it
// is an allocation.
func TestDecodeChecksTheCountBeforeAllocating(t *testing.T) {
	t.Parallel()
	b := wire.Append(nil, wire.Snapshot{Tick: 1})
	b[len(b)-2], b[len(b)-1] = 0xFF, 0xFF // 65535 actors, zero bytes of them
	if _, err := wire.Decode(b); !errors.Is(err, wire.ErrShort) {
		t.Errorf("err = %v, want ErrShort", err)
	}
	b = wire.Append(nil, wire.You{Tick: 1})
	b[len(b)-1] = 255
	if _, err := wire.Decode(b); !errors.Is(err, wire.ErrShort) {
		t.Errorf("err = %v, want ErrShort", err)
	}
}

// FuzzDecode holds the decoder to the rule the sheet scanner is held to:
// whatever it accepts is safe to apply, and whatever it refuses is refused
// with one of the four errors.
func FuzzDecode(f *testing.F) {
	for _, m := range every() {
		f.Add(wire.Append(nil, m))
	}
	f.Add([]byte{})
	f.Add([]byte{byte(wire.KindSnapshot), 0, 0, 0, 0, 0xFF, 0xFF})
	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := wire.Decode(b)
		if err != nil {
			if !errors.Is(err, wire.ErrKind) && !errors.Is(err, wire.ErrShort) &&
				!errors.Is(err, wire.ErrLong) && !errors.Is(err, wire.ErrValue) {
				t.Fatalf("an error that is not one of the four: %v", err)
			}
			return
		}
		// Accepted, so it has to be the whole message and nothing else: the
		// bytes back out are the bytes that came in.
		if again := wire.Append(nil, m); string(again) != string(b) {
			t.Fatalf("re-encoding %T changed the bytes:\n in %x\nout %x", m, b, again)
		}
		finite := func(v float32) bool { return v == v && v <= math.MaxFloat32 && v >= -math.MaxFloat32 }
		switch m := m.(type) {
		case wire.Spawn:
			if !finite(m.X) || !finite(m.Y) {
				t.Fatalf("accepted a Spawn at (%v, %v)", m.X, m.Y)
			}
		case wire.Event:
			if !finite(m.X) || !finite(m.Y) {
				t.Fatalf("accepted an Event at (%v, %v)", m.X, m.Y)
			}
		case wire.Snapshot:
			for _, a := range m.Actors {
				if !finite(a.X) || !finite(a.Y) {
					t.Fatalf("accepted an actor at (%v, %v)", a.X, a.Y)
				}
			}
		}
	})
}
