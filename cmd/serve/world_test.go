package main

import (
	"errors"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/andygeiss/wisp-engine"
	"github.com/andygeiss/wisp-engine/internal/lab"
	"github.com/andygeiss/wisp-engine/internal/wire"
	"github.com/andygeiss/wisp-engine/internal/ws"
)

// quiet is the logger every test hands the server.
var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// The world is tested the way it runs: real sockets through the real routes,
// over net.Pipe rather than a port, inside a synctest bubble so that "the
// reader has posted the join" is synctest.Wait rather than a sleep, and the
// tests call Tick themselves so there is no clock to wait on.

// pipeListener hands one end of a net.Pipe to the http.Server for every
// dial.
type pipeListener struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{conns: make(chan net.Conn), done: make(chan struct{})}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error   { l.once.Do(func() { close(l.done) }); return nil }
func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

func (l *pipeListener) dial(t *testing.T) net.Conn {
	t.Helper()
	client, server := net.Pipe()
	select {
	case l.conns <- server:
	case <-l.done:
		t.Fatal("the server is gone")
	}
	return client
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// arena is a world served over pipes, with no clock.
type arena struct {
	t   *testing.T
	w   *world
	ln  *pipeListener
	srv *http.Server
}

func newArena(t *testing.T, players, crowd int) *arena {
	t.Helper()
	w := newWorld(Config{Bouncers: 100, Crowd: crowd, Players: players, Tick: 30}, quiet)
	handler, err := routes(newTree(t, "\x00asm"), "v", quiet, w)
	if err != nil {
		t.Fatal(err)
	}
	a := &arena{t: t, w: w, ln: newPipeListener(), srv: &http.Server{Handler: handler}}
	go a.srv.Serve(a.ln)
	return a
}

func (a *arena) close() { a.srv.Close() }

// player is a connected client. pump decodes everything the server sends into
// in, and done closes when the socket does.
type player struct {
	conn    *ws.Conn
	in      chan wire.Message
	done    chan struct{}
	err     error
	reading bool
}

// dial connects and says nothing.
func (a *arena) dial() *player {
	a.t.Helper()
	conn, err := ws.Dial(a.ln.dial(a.t), "lab.test", "/ws")
	if err != nil {
		a.t.Fatal(err)
	}
	return &player{conn: conn, in: make(chan wire.Message, 1<<14), done: make(chan struct{})}
}

// join connects, reads, and says hello.
func (a *arena) join() *player {
	a.t.Helper()
	p := a.dial()
	p.reading = true
	go p.pump()
	p.send(a.t, wire.Hello{Version: wire.Version})
	return p
}

func (p *player) pump() {
	defer close(p.done)
	for {
		b, err := p.conn.Read()
		if err != nil {
			p.err = err
			return
		}
		m, err := wire.Decode(b)
		if err != nil {
			p.err = err
			return
		}
		p.in <- m
	}
}

func (p *player) send(t *testing.T, m wire.Message) {
	t.Helper()
	if err := p.conn.Write(wire.Append(nil, m)); err != nil {
		t.Fatalf("sending %T: %v", m, err)
	}
}

func (p *player) close() {
	p.conn.Close(ws.CloseNormal, "")
	if p.reading {
		<-p.done
	}
}

// closed waits for the socket to end and returns the close code, or 0.
func (p *player) closed(t *testing.T) uint16 {
	t.Helper()
	<-p.done
	if ce, ok := errors.AsType[*ws.CloseError](p.err); ok {
		return ce.Code
	}
	t.Logf("the socket ended with %v", p.err)
	return 0
}

// frame reads one tick's worth of messages: everything up to and including
// the Snapshot that ends a tick's batch.
func frame(t *testing.T, p *player) []wire.Message {
	t.Helper()
	var msgs []wire.Message
	for {
		select {
		case m := <-p.in:
			msgs = append(msgs, m)
			if _, ok := m.(wire.Snapshot); ok {
				return msgs
			}
			continue
		default:
		}
		select {
		case m := <-p.in:
			msgs = append(msgs, m)
			if _, ok := m.(wire.Snapshot); ok {
				return msgs
			}
		case <-p.done:
			t.Fatalf("the socket closed (%v) after %d messages of the tick", p.err, len(msgs))
		}
	}
}

// pick returns the messages of one type, in order.
func pick[T wire.Message](msgs []wire.Message) []T {
	var out []T
	for _, m := range msgs {
		if v, ok := m.(T); ok {
			out = append(out, v)
		}
	}
	return out
}

// one returns the single message of a type in a tick, and fails otherwise.
func one[T wire.Message](t *testing.T, msgs []wire.Message) T {
	t.Helper()
	got := pick[T](msgs)
	if len(got) != 1 {
		var zero T
		t.Fatalf("%d %T in the tick, want 1: %v", len(got), zero, msgs)
	}
	return got[0]
}

// actor finds slot in a snapshot.
func actor(t *testing.T, s wire.Snapshot, slot uint16) wire.Actor {
	t.Helper()
	for _, a := range s.Actors {
		if a.Slot == slot {
			return a
		}
	}
	t.Fatalf("slot %d is not in the snapshot %v", slot, s.Actors)
	return wire.Actor{}
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

// joined joins n players, ticks once, and returns each with its first tick.
func joined(t *testing.T, a *arena, n int) (players []*player, first [][]wire.Message) {
	t.Helper()
	for range n {
		players = append(players, a.join())
	}
	synctest.Wait()
	a.w.Tick()
	for _, p := range players {
		first = append(first, frame(t, p))
	}
	return players, first
}

func TestTwoPlayersSeeEachOther(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		a := newArena(t, 2, 0)
		defer a.close()
		players, first := joined(t, a, 2)
		defer players[0].close()
		defer players[1].close()

		w1, w2 := one[wire.Welcome](t, first[0]), one[wire.Welcome](t, first[1])
		if w1.You == w2.You {
			t.Fatalf("both players are slot %d", w1.You)
		}
		if w1.Version != wire.Version || w1.TickRate != 30 || w1.WorldW != lab.WorldW || w1.WorldH != lab.WorldH {
			t.Errorf("Welcome = %+v", w1)
		}
		for i, f := range first {
			spawns := pick[wire.Spawn](f)
			if len(spawns) != 2 {
				t.Fatalf("player %d got %d spawns before the first snapshot, want both players", i, len(spawns))
			}
			if !(spawns[0].Slot == w1.You && spawns[1].Slot == w2.You) {
				t.Errorf("player %d got spawns for %d and %d, want %d then %d", i, spawns[0].Slot, spawns[1].Slot, w1.You, w2.You)
			}
			if s := spawns[0]; s.Width != lab.HeroW || s.State&wisp.StateVisible == 0 || s.Alpha != 255 || s.Z != lab.ZPlayer {
				t.Errorf("a player's spawn is %+v", s)
			}
			snap := one[wire.Snapshot](t, f)
			if len(snap.Actors) != 2 {
				t.Errorf("player %d's first snapshot has %d actors, want 2", i, len(snap.Actors))
			}
			you := one[wire.You](t, f)
			if len(you.Cooldowns) != int(lab.NumSkills) {
				t.Errorf("You carries %d cooldowns, want %d", len(you.Cooldowns), lab.NumSkills)
			}
		}
	})
}

func TestAnIntentMovesThePlayerBySpeedTimesStep(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		a := newArena(t, 1, 0)
		defer a.close()
		players, first := joined(t, a, 1)
		p := players[0]
		defer p.close()
		you := one[wire.Welcome](t, first[0]).You
		step := float32(a.w.e.World.Speed * a.w.step)

		p.send(t, wire.Intent{Seq: 1, DX: 1})
		synctest.Wait()
		a.w.Tick()
		me := actor(t, one[wire.Snapshot](t, frame(t, p)), you)
		if !near(me.X, lab.WorldW/2+step) || me.Y != lab.WorldH/2 {
			t.Errorf("after one tick right the player is at (%v, %v), want (%v, %v)", me.X, me.Y, lab.WorldW/2+step, lab.WorldH/2)
		}
		if me.State&wisp.StateMoveRight == 0 || me.State&wisp.StateMove == 0 || me.Row != uint8(lab.Tag(lab.HeroWalk, lab.East)) {
			t.Errorf("a moving player has state %#x row %d", me.State, me.Row)
		}

		// The axis holds until the next intent says otherwise.
		a.w.Tick()
		me = actor(t, one[wire.Snapshot](t, frame(t, p)), you)
		if !near(me.X, lab.WorldW/2+2*step) {
			t.Errorf("after a second tick with the axis held x = %v, want %v", me.X, lab.WorldW/2+2*step)
		}

		p.send(t, wire.Intent{Seq: 2})
		synctest.Wait()
		a.w.Tick()
		me = actor(t, one[wire.Snapshot](t, frame(t, p)), you)
		if !near(me.X, lab.WorldW/2+2*step) || me.State&wisp.StateIdle == 0 || me.Row != uint8(lab.Tag(lab.HeroIdle, lab.East)) {
			t.Errorf("after letting go x = %v state %#x row %d, want it idle where it was", me.X, me.State, me.Row)
		}
	})
}

// The cheat check: an axis is a direction, not a speed.
func TestAnAxisOfAHundredMovesAtOne(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		a := newArena(t, 1, 0)
		defer a.close()
		players, first := joined(t, a, 1)
		p := players[0]
		defer p.close()
		you := one[wire.Welcome](t, first[0]).You

		p.send(t, wire.Intent{Seq: 1, DX: 100, DY: -100})
		synctest.Wait()
		a.w.Tick()
		me := actor(t, one[wire.Snapshot](t, frame(t, p)), you)
		d := float32(a.w.e.World.Speed * a.w.step / math.Sqrt2)
		if !near(me.X, lab.WorldW/2+d) || !near(me.Y, lab.WorldH/2-d) {
			t.Errorf("an axis of (100, -100) moved the player to (%v, %v), want (%v, %v)", me.X, me.Y, lab.WorldW/2+d, lab.WorldH/2-d)
		}
	})
}

func TestAStrikeDespawnsForEverybody(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		a := newArena(t, 2, 0)
		defer a.close()
		players, first := joined(t, a, 2)
		p1, p2 := players[0], players[1]
		defer p1.close()
		defer p2.close()
		you := one[wire.Welcome](t, first[0]).You

		// E: ten bouncers around the player, announced to both.
		p1.send(t, wire.Intent{Seq: 1, Skills: 1 << lab.Spawn})
		synctest.Wait()
		a.w.Tick()
		f1, f2 := frame(t, p1), frame(t, p2)
		for i, f := range [][]wire.Message{f1, f2} {
			ev := one[wire.Event](t, f)
			if ev.Slot != you || ev.Skill != uint8(lab.Spawn) {
				t.Errorf("player %d got the event %+v", i, ev)
			}
			if n := len(pick[wire.Spawn](f)); n != lab.SpawnBatch {
				t.Errorf("player %d got %d spawns, want %d", i, n, lab.SpawnBatch)
			}
			if n := len(one[wire.Snapshot](t, f).Actors); n != 2+lab.SpawnBatch {
				t.Errorf("player %d's snapshot has %d actors, want %d", i, n, 2+lab.SpawnBatch)
			}
		}
		if cd := one[wire.You](t, f1).Cooldowns[lab.Spawn]; cd == 0 || cd > uint16(lab.Spawn.Cooldown()) {
			t.Errorf("the striker's spawn cooldown reads %d", cd)
		}
		if cd := one[wire.You](t, f2).Cooldowns[lab.Spawn]; cd != 0 {
			t.Errorf("the other player's spawn cooldown reads %d, want 0", cd)
		}

		// The world is the test's between ticks: park one bouncer on the
		// player and the rest far away, still, so exactly one is in reach.
		e := a.w.e
		var onTop, away []uint16
		for i, s := range pick[wire.Spawn](f1) {
			a.w.bouncers.SetVelocity(int(s.Slot), 0, 0)
			if i == 0 {
				e.Place(int(s.Slot), e.X[you]+2, e.Y[you])
				onTop = append(onTop, s.Slot)
				continue
			}
			e.Place(int(s.Slot), 48, 48)
			away = append(away, s.Slot)
		}

		// Q: the one in reach goes, for both.
		p1.send(t, wire.Intent{Seq: 2, Skills: 1 << lab.Strike})
		synctest.Wait()
		a.w.Tick()
		for i, p := range players {
			f := frame(t, p)
			if ev := one[wire.Event](t, f); ev.Skill != uint8(lab.Strike) {
				t.Errorf("player %d got the event %+v", i, ev)
			}
			gone := pick[wire.Despawn](f)
			if len(gone) != 1 || gone[0].Slot != onTop[0] {
				t.Errorf("player %d saw %v despawn, want just %v", i, gone, onTop)
			}
			if n := len(one[wire.Snapshot](t, f).Actors); n != 2+len(away) {
				t.Errorf("player %d's snapshot has %d actors, want %d", i, n, 2+len(away))
			}
		}

		// Q held down: a press every tick inside the cooldown does nothing.
		for range 3 {
			p1.send(t, wire.Intent{Seq: 3, Skills: 1 << lab.Strike})
			synctest.Wait()
			a.w.Tick()
			f := frame(t, p1)
			if n := len(pick[wire.Event](f)) + len(pick[wire.Despawn](f)); n != 0 {
				t.Fatalf("a strike inside the cooldown produced %d messages", n)
			}
		}
	})
}

func TestAClientThatStopsReadingIsClosed(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		a := newArena(t, 2, 0)
		defer a.close()
		p := a.join()
		defer p.close()
		slow := a.dial() // never reads
		defer slow.close()
		slow.send(t, wire.Hello{Version: wire.Version})
		synctest.Wait()
		a.w.Tick()
		first := frame(t, p)
		you := one[wire.Welcome](t, first).You
		var slowSlot uint16
		for _, s := range pick[wire.Spawn](first) {
			if s.Slot != you {
				slowSlot = s.Slot
			}
		}

		// The slow one's queue fills a batch a tick, and the world moves on.
		for range outboundQueue + 2 {
			a.w.Tick()
		}
		// Its writer is stuck on a socket nobody reads; the write deadline
		// ends that, the socket closes, its reader posts the leave, and the
		// next tick tells everybody.
		synctest.Sleep(3 * writeTimeout)
		synctest.Wait()
		a.w.Tick()

		found := false
		for range outboundQueue + 4 {
			if len(pick[wire.Despawn](frame(t, p))) == 1 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("the player never saw slot %d leave", slowSlot)
		}
		if a.w.e.Live(int(slowSlot)) {
			t.Error("the slow player's entity is still in the world")
		}
	})
}

func TestShutdownClosesEveryoneWithGoingAway(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		a := newArena(t, 2, 0)
		defer a.close()
		players, _ := joined(t, a, 2)

		a.w.hub.closeAll()
		a.w.hub.Wait()

		for i, p := range players {
			if code := p.closed(t); code != ws.CloseGoingAway {
				t.Errorf("player %d was closed with %d, want 1001", i, code)
			}
		}
		// Nobody gets in after.
		late := a.dial()
		defer late.close()
		late.reading = true
		go late.pump()
		if code := late.closed(t); code != ws.CloseGoingAway {
			t.Errorf("a socket after shutdown was closed with %d, want 1001", code)
		}
	})
}

func TestTheServerRefuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		first wire.Message
	}{
		{"an intent before hello", wire.Intent{DX: 1}},
		{"a hello from the future", wire.Hello{Version: wire.Version + 1}},
		{"a server message from a client", wire.Welcome{Version: wire.Version}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				a := newArena(t, 1, 0)
				defer a.close()
				p := a.dial()
				defer p.close()
				p.reading = true
				go p.pump()
				p.send(t, tt.first)
				if code := p.closed(t); code != ws.ClosePolicy {
					t.Errorf("closed with %d, want 1008", code)
				}
			})
		})
	}

	t.Run("a third player when two is the most", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			a := newArena(t, 2, 0)
			defer a.close()
			players, _ := joined(t, a, 2)
			defer players[0].close()
			defer players[1].close()
			third := a.join()
			defer third.close()
			synctest.Wait()
			a.w.Tick()
			if code := third.closed(t); code != ws.ClosePolicy {
				t.Errorf("the third player was closed with %d, want 1008", code)
			}
		})
	})

	t.Run("more than 120 messages a second", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			a := newArena(t, 1, 0)
			defer a.close()
			players, _ := joined(t, a, 1)
			p := players[0]
			defer p.close()
			for i := range maxMessagesPerSecond + 1 {
				if err := p.conn.Write(wire.Append(nil, wire.Ping{T: uint32(i)})); err != nil {
					break
				}
			}
			if code := p.closed(t); code != ws.ClosePolicy {
				t.Errorf("closed with %d, want 1008", code)
			}
		})
	})
}

func TestAPingIsAnsweredWithTheTick(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		a := newArena(t, 1, 0)
		defer a.close()
		players, _ := joined(t, a, 1)
		p := players[0]
		defer p.close()
		a.w.Tick()
		frame(t, p)

		p.send(t, wire.Ping{T: 4242})
		select {
		case m := <-p.in:
			pong, ok := m.(wire.Pong)
			if !ok || pong.T != 4242 || pong.Tick != a.w.tick.Load() {
				t.Errorf("got %+v, want a Pong echoing 4242 at tick %d", m, a.w.tick.Load())
			}
		case <-p.done:
			t.Fatalf("the socket closed: %v", p.err)
		}
	})
}
