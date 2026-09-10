package replica_test

import (
	"errors"
	"testing"

	"github.com/andygeiss/wisp-engine"
	"github.com/andygeiss/wisp-engine/internal/replica"
	"github.com/andygeiss/wisp-engine/internal/wire"
)

// The engine's own tick is what applies a replica: Simulate is Tick.
func newReplica(t *testing.T) (*wisp.Engine, *replica.Replica) {
	t.Helper()
	e := wisp.New(wisp.Config{})
	e.Time.TickRate = 30
	r := replica.New(e)
	e.Simulate = func(float64) { r.Tick() }
	return e, r
}

const step = 1000.0 / 30

func spawn(slot uint16, x, y float32) wire.Spawn {
	return wire.Spawn{
		Slot: slot, Width: 32, Height: 32, X: x, Y: y, Z: 1, Alpha: 255, Row: 2,
		State: wisp.StateAnimated | wisp.StateAnimatedLoop | wisp.StateVisible | wisp.StateIdle | wisp.StateFaceRight,
	}
}

func snapshot(tick uint32, actors ...wire.Actor) wire.Snapshot {
	return wire.Snapshot{Tick: tick, Actors: actors}
}

func TestSpawnSnapshotAndDespawnReachTheEngine(t *testing.T) {
	t.Parallel()
	e, r := newReplica(t)
	r.Push(wire.Welcome{Version: wire.Version, TickRate: 30, WorldW: 1280, WorldH: 768, You: 7, Tick: 1})
	r.Push(spawn(7, 100, 100))
	r.Push(spawn(9, 200, 200))
	r.Push(wire.You{Tick: 1, Cooldowns: []uint16{1000, 0, 0}})
	r.Push(snapshot(1, wire.Actor{Slot: 7, X: 104, Y: 100, State: wisp.StateVisible | wisp.StateMove | wisp.StateMoveRight, Row: 2},
		wire.Actor{Slot: 9, X: 200, Y: 200, State: wisp.StateVisible, Row: 3}))

	if r.You() != -1 || r.Welcomed {
		t.Fatal("the replica applied something before a tick")
	}
	e.Tick(step)

	if !r.Welcomed || r.TickRate != 30 || r.WorldW != 1280 || r.YouSlot != 7 {
		t.Errorf("Welcome was not applied: %+v", r)
	}
	you := r.You()
	if you < 0 || you != r.Index(7) {
		t.Fatalf("You() = %d, Index(7) = %d", you, r.Index(7))
	}
	if e.Count() != 2 {
		t.Fatalf("%d entities, want 2", e.Count())
	}
	if e.X[you] != 104 || e.Y[you] != 100 {
		t.Errorf("you are at (%v, %v), want the snapshot's (104, 100)", e.X[you], e.Y[you])
	}
	if e.State[you]&wisp.StateRemote == 0 || e.State[you]&wisp.StateMoveRight == 0 {
		t.Errorf("your state is %#x, want the server's bits plus StateRemote", e.State[you])
	}
	other := r.Index(9)
	if e.ImageRow[other] != 3 || e.Alpha[other] != 1 || e.SpriteWidth[other] != 32 || e.Z[other] != 1 {
		t.Errorf("the other entity was spawned as row %d alpha %v width %v z %d", e.ImageRow[other], e.Alpha[other], e.SpriteWidth[other], e.Z[other])
	}
	if len(r.Cooldowns) != 3 || r.Cooldowns[0] != 1000 {
		t.Errorf("Cooldowns = %v", r.Cooldowns)
	}
	if r.Applied != 1 || r.LastTick != 1 || r.Depth() != 0 || r.Pending() != 0 {
		t.Errorf("applied %d, last tick %d, depth %d, pending %d", r.Applied, r.LastTick, r.Depth(), r.Pending())
	}

	r.Push(wire.Despawn{Slot: 9})
	r.Push(snapshot(2, wire.Actor{Slot: 7, X: 108, Y: 100, State: wisp.StateVisible, Row: 2}))
	e.Tick(step)
	if e.Live(other) || r.Index(9) != -1 || e.Count() != 1 {
		t.Error("the despawned entity is still there")
	}
}

// The engine does not move a remote entity between snapshots, and a tick with
// no snapshot leaves it exactly where the last one put it.
func TestARemoteEntityGoesOnlyWhereTheServerSaysAndHoldsWhenItIsQuiet(t *testing.T) {
	t.Parallel()
	e, r := newReplica(t)
	r.Push(spawn(1, 100, 100))
	r.Push(snapshot(1, wire.Actor{Slot: 1, X: 100, Y: 100, State: wisp.StateVisible | wisp.StateMoveRight, Row: 2}))
	e.Tick(step)
	i := r.Index(1)

	e.Tick(step) // nothing queued
	if e.X[i] != 100 || e.PrevX[i] != 100 {
		t.Errorf("a quiet tick moved the entity to %v from %v", e.X[i], e.PrevX[i])
	}
	if r.Held != 1 {
		t.Errorf("Held = %d, want 1", r.Held)
	}
	if e.FrameOffset[i] == 0 && e.FrameTime[i] == 0 {
		// Not a failure in itself: two ticks may not cross a frame. The point
		// is only that animation is the client's, checked below.
		t.Log("the animation has not advanced yet")
	}
}

// Two snapshots on two ticks are the pair the draw blends between.
func TestTwoSnapshotsAreTheBlendsPair(t *testing.T) {
	t.Parallel()
	e, r := newReplica(t)
	r.Push(spawn(1, 100, 100))
	r.Push(snapshot(1, wire.Actor{Slot: 1, X: 100, Y: 100, State: wisp.StateVisible, Row: 2}))
	r.Push(snapshot(2, wire.Actor{Slot: 1, X: 110, Y: 100, State: wisp.StateVisible, Row: 2}))

	// Two ticks and a half, a frame each because Step caps a frame at
	// MaxStep: the first two apply the two snapshots, the half leaves the
	// frame between the second and the one that has not come.
	e.Step(step)
	e.Step(step)
	e.Step(step / 2)
	i := r.Index(1)
	if e.PrevX[i] != 100 || e.X[i] != 110 {
		t.Fatalf("prev %v, now %v; want the two snapshots 100 and 110", e.PrevX[i], e.X[i])
	}
	if x, _ := e.DrawPos(i); x != 105 {
		t.Errorf("drawn at %v half a tick in, want 105, halfway between the two", x)
	}
	if r.Depth() != 0 || r.Held != 0 {
		t.Errorf("depth %d held %d after two snapshots in two ticks", r.Depth(), r.Held)
	}
}

func TestTheDepthRule(t *testing.T) {
	t.Parallel()
	queued := func(t *testing.T, n int) (*wisp.Engine, *replica.Replica) {
		t.Helper()
		e, r := newReplica(t)
		r.Push(spawn(1, 0, 0))
		for k := 1; k <= n; k++ {
			r.Push(snapshot(uint32(k), wire.Actor{Slot: 1, X: float32(k), State: wisp.StateVisible}))
		}
		return e, r
	}

	t.Run("one or two waiting applies one", func(t *testing.T) {
		t.Parallel()
		e, r := queued(t, 2)
		e.Tick(step)
		if r.LastTick != 1 || r.Depth() != 1 || r.FastForwards != 0 {
			t.Errorf("last %d depth %d fast-forwards %d, want 1, 1, 0", r.LastTick, r.Depth(), r.FastForwards)
		}
	})
	t.Run("three waiting applies two", func(t *testing.T) {
		t.Parallel()
		e, r := queued(t, 3)
		e.Tick(step)
		if r.LastTick != 2 || r.Depth() != 1 || r.FastForwards != 1 {
			t.Errorf("last %d depth %d fast-forwards %d, want 2, 1, 1", r.LastTick, r.Depth(), r.FastForwards)
		}
		// A fast-forward is still a blend: the pair is snapshots 0 and 2.
		i := r.Index(1)
		if e.PrevX[i] != 0 || e.X[i] != 2 {
			t.Errorf("prev %v now %v, want 0 and 2", e.PrevX[i], e.X[i])
		}
	})
	t.Run("a tab that slept applies everything", func(t *testing.T) {
		t.Parallel()
		e, r := queued(t, 70)
		e.Tick(step)
		if r.LastTick != 70 || r.Depth() != 0 || r.CatchUps != 1 || r.FastForwards != 0 || r.Pending() != 0 {
			t.Errorf("last %d depth %d catch-ups %d fast-forwards %d pending %d", r.LastTick, r.Depth(), r.CatchUps, r.FastForwards, r.Pending())
		}
	})
	t.Run("two waiting for a second applies two once", func(t *testing.T) {
		t.Parallel()
		e, r := queued(t, 2)
		// One applied and one arrived a tick, so every tick finds two: what
		// a fast-forward leaves behind, or a clock that drifted. Thirty is a
		// second at the wire's 30 Hz.
		for k := 1; k < 30; k++ {
			e.Tick(step)
			if r.LastDepth != 2 || r.Depth() != 1 || r.Drains != 0 {
				t.Fatalf("tick %d found %d, left %d, drains %d; want 2, 1, 0", k, r.LastDepth, r.Depth(), r.Drains)
			}
			r.Push(snapshot(uint32(k+2), wire.Actor{Slot: 1, X: float32(k + 2), State: wisp.StateVisible}))
		}
		e.Tick(step)
		if r.LastDepth != 2 || r.Depth() != 0 || r.Drains != 1 || r.FastForwards != 0 {
			t.Fatalf("the thirtieth tick found %d, left %d, drains %d, fast-forwards %d; want 2, 0, 1, 0", r.LastDepth, r.Depth(), r.Drains, r.FastForwards)
		}
		// From here on a tick finds the one it likes, and nothing holds.
		r.Push(snapshot(32, wire.Actor{Slot: 1, X: 32, State: wisp.StateVisible}))
		e.Tick(step)
		if r.LastDepth != 1 || r.Depth() != 0 || r.Held != 0 || r.Drains != 1 {
			t.Errorf("after the drain a tick found %d, left %d, held %d, drains %d; want 1, 0, 0, 1", r.LastDepth, r.Depth(), r.Held, r.Drains)
		}
	})
	t.Run("an early snapshot is two for one tick and starts the count over", func(t *testing.T) {
		t.Parallel()
		e, r := queued(t, 1)
		next := uint32(2)
		arrive := func() {
			r.Push(snapshot(next, wire.Actor{Slot: 1, X: float32(next), State: wisp.StateVisible}))
			next++
		}
		tick := func(want int) {
			t.Helper()
			e.Tick(step)
			if r.LastDepth != want || r.Drains != 0 {
				t.Fatalf("before snapshot %d the tick found %d, drains %d; want %d, 0", next, r.LastDepth, r.Drains, want)
			}
		}
		// At rest one arrives and the tick finds it. An early one lands with
		// its predecessor still waiting, and the tick after it has nothing
		// new.
		tick(1)
		arrive()
		arrive()
		tick(2)
		tick(1)
		// Twenty-nine at two, one at one, twenty-nine at two: the count
		// starts over at the one, so the thirtieth of the second run is the
		// first to take anything back.
		arrive()
		for range 29 {
			arrive()
			tick(2)
		}
		tick(1)
		arrive()
		for range 29 {
			arrive()
			tick(2)
		}
		arrive()
		e.Tick(step)
		if r.LastDepth != 2 || r.Depth() != 0 || r.Drains != 1 || r.FastForwards != 0 || r.Held != 0 {
			t.Errorf("the thirtieth in a row found %d, left %d, drains %d, fast-forwards %d, held %d; want 2, 0, 1, 0, 0", r.LastDepth, r.Depth(), r.Drains, r.FastForwards, r.Held)
		}
	})
	t.Run("nothing waiting before the first snapshot is loading, not a hold", func(t *testing.T) {
		t.Parallel()
		e, r := newReplica(t)
		r.Push(spawn(1, 0, 0))
		e.Tick(step)
		if r.Held != 0 || r.Index(1) != -1 {
			t.Errorf("held %d, index %d: a spawn was applied without its snapshot, or counted as a hold", r.Held, r.Index(1))
		}
	})
}

func TestMessagesApplyInTheOrderTheyArrived(t *testing.T) {
	t.Parallel()
	e, r := newReplica(t)
	r.Push(spawn(1, 10, 10))
	r.Push(snapshot(1, wire.Actor{Slot: 1, X: 10, Y: 10, State: wisp.StateVisible}))
	r.Push(wire.Despawn{Slot: 1})
	r.Push(spawn(1, 500, 500)) // the server reused the slot
	r.Push(snapshot(2, wire.Actor{Slot: 1, X: 501, Y: 500, State: wisp.StateVisible}))

	e.Tick(step)
	first := r.Index(1)
	e.Tick(step)
	second := r.Index(1)

	if e.Count() != 1 {
		t.Fatalf("%d entities, want the one the slot now names", e.Count())
	}
	if e.X[second] != 501 || e.PrevX[second] != 500 {
		t.Errorf("the reused slot is at %v from %v, want 501 from its own spawn at 500", e.X[second], e.PrevX[second])
	}
	if first == second && !e.Live(first) {
		t.Error("the slot maps to a dead entity")
	}
}

func TestASpawnForASlotThatExistsReplacesIt(t *testing.T) {
	t.Parallel()
	e, r := newReplica(t)
	r.Push(spawn(1, 10, 10))
	r.Push(spawn(1, 20, 20))
	r.Push(snapshot(1))
	e.Tick(step)
	if e.Count() != 1 || e.X[r.Index(1)] != 20 {
		t.Errorf("%d entities, slot 1 at %v; want one entity at 20", e.Count(), e.X[r.Index(1)])
	}
}

func TestAnUnknownSlotIsIgnored(t *testing.T) {
	t.Parallel()
	e, r := newReplica(t)
	r.Push(snapshot(1, wire.Actor{Slot: 42, X: 1, Y: 1, State: wisp.StateVisible}))
	r.Push(wire.Despawn{Slot: 42})
	e.Tick(step)
	if e.Count() != 0 || r.Index(42) != -1 {
		t.Error("an actor nobody spawned got in")
	}
}

func TestARowChangeStartsTheAnimationOver(t *testing.T) {
	t.Parallel()
	e, r := newReplica(t)
	r.Push(spawn(1, 0, 0))
	r.Push(snapshot(1, wire.Actor{Slot: 1, State: wisp.StateVisible, Row: 2}))
	e.Tick(step)
	i := r.Index(1)
	e.FrameOffset[i], e.FrameTime[i] = 3, 40

	r.Push(snapshot(2, wire.Actor{Slot: 1, State: wisp.StateVisible, Row: 2}))
	e.Tick(step)
	if e.FrameOffset[i] != 3 || e.FrameTime[i] != 40 {
		t.Errorf("the same row reset the frame to %d at %v ms", e.FrameOffset[i], e.FrameTime[i])
	}
	r.Push(snapshot(3, wire.Actor{Slot: 1, State: wisp.StateVisible, Row: 3}))
	e.Tick(step)
	if e.ImageRow[i] != 3 || e.FrameOffset[i] != 0 || e.FrameTime[i] != 0 {
		t.Errorf("a new row left row %d frame %d at %v ms", e.ImageRow[i], e.FrameOffset[i], e.FrameTime[i])
	}
}

func TestEventsReachTheCallbackInOrder(t *testing.T) {
	t.Parallel()
	e, r := newReplica(t)
	var got []uint8
	r.OnEvent = func(ev wire.Event) { got = append(got, ev.Skill) }
	r.Push(wire.Event{Tick: 1, Slot: 1, Skill: 2})
	r.Push(wire.Event{Tick: 1, Slot: 1, Skill: 0})
	r.Push(snapshot(1))
	r.Push(wire.Event{Tick: 2, Slot: 1, Skill: 1}) // next tick's
	e.Tick(step)
	if len(got) != 2 || got[0] != 2 || got[1] != 0 {
		t.Errorf("events applied = %v, want [2 0], the ones before the snapshot", got)
	}
}

func TestAWelcomeFromAnotherVersionIsAnError(t *testing.T) {
	t.Parallel()
	e, r := newReplica(t)
	r.Push(wire.Welcome{Version: wire.Version + 1, TickRate: 30})
	r.Push(snapshot(1))
	e.Tick(step)
	if !errors.Is(r.Err, replica.ErrVersion) || r.Welcomed {
		t.Errorf("Err = %v, Welcomed = %v", r.Err, r.Welcomed)
	}
}

func TestAPongIsNotTheWorlds(t *testing.T) {
	t.Parallel()
	_, r := newReplica(t)
	r.Push(wire.Pong{T: 1, Tick: 2})
	if r.Pending() != 0 {
		t.Error("a Pong was queued for the world")
	}
}

// A skill of your own plays a light hit stop, which holds the engine for
// five frames at 60 Hz: two and a half server ticks of snapshots pile up. The
// fast-forward takes them down to two, the drain takes the last one back
// within a second, and the depth the tick found — the number the HUD prints
// — never flips between the two frames of one tick, where the live queue
// does whenever the snapshot lands in the first half of a tick.
func TestAHitStopIsPaidBackToOneSnapshot(t *testing.T) {
	t.Parallel()
	const frame = 1000.0 / 60
	flipped := false
	for _, phase := range []float64{step / 4, 3 * step / 4} {
		e, r := newReplica(t)
		r.Push(spawn(1, 0, 0))
		next, tick := phase, uint32(1)
		arrive := func(now float64) {
			for next <= now {
				r.Push(snapshot(tick, wire.Actor{Slot: 1, X: float32(tick), State: wisp.StateVisible}))
				tick++
				next += step
			}
		}
		now := 0.0
		run := func(frames int, each func()) {
			for range frames {
				arrive(now)
				e.Step(frame)
				now += frame
				if each != nil {
					each()
				}
			}
		}

		run(120, nil) // two seconds to settle
		if r.LastDepth != 1 || r.Held != 0 || r.FastForwards != 0 || r.Drains != 0 {
			t.Fatalf("phase %v: at rest the tick found %d, held %d, fast-forwards %d, drains %d; want 1, 0, 0, 0", phase, r.LastDepth, r.Held, r.FastForwards, r.Drains)
		}

		e.Impact(e.Feel.Light)
		// The stop is five frames, the fast-forward two or four, and the
		// second at two sixty: the drain has fired by seventy. A second and
		// a half leaves room.
		run(90, nil)
		found, live := map[int]bool{}, map[int]bool{}
		run(60, func() {
			found[r.LastDepth] = true
			live[r.Depth()] = true
		})
		if r.Drains != 1 || r.FastForwards == 0 || r.Held != 0 {
			t.Errorf("phase %v: after the stop drains %d, fast-forwards %d, held %d; want 1, some, 0", phase, r.Drains, r.FastForwards, r.Held)
		}
		if len(found) != 1 || !found[1] {
			t.Errorf("phase %v: in the second after the stop was paid back the tick found %v, want only 1", phase, found)
		}
		if len(live) > 1 {
			flipped = true
		}
	}
	if !flipped {
		t.Error("the live queue never flipped between two frames of one tick, so the HUD would not have needed LastDepth")
	}
}
