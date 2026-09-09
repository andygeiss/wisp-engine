package lab_test

import (
	"slices"
	"testing"

	"github.com/andygeiss/wisp-engine"
	"github.com/andygeiss/wisp-engine/internal/lab"
)

func newEngine(t *testing.T) *wisp.Engine {
	t.Helper()
	e := wisp.New(wisp.Config{})
	lab.Setup(e)
	return e
}

func TestTheSheetNamesTheRows(t *testing.T) {
	t.Parallel()
	s := lab.Sheet()
	if got := s.Lookup(lab.TagRun); got != lab.RowMoveRight {
		t.Errorf("Lookup(%q) = %d, want the row %d", lab.TagRun, got, lab.RowMoveRight)
	}
	if len(s.Tags) != lab.SheetRows || len(s.Frames) != lab.SheetRows*lab.SheetCols {
		t.Errorf("the sheet has %d tags and %d frames", len(s.Tags), len(s.Frames))
	}

	// A player who walks right is drawn from the run-right row.
	e := newEngine(t)
	p := lab.NewPlayer(e)
	e.Move(p.Entity, 1, 0)
	e.Tick(16)
	if e.ImageRow[p.Entity] != lab.RowMoveRight {
		t.Errorf("a player walking right is on row %d, want %d", e.ImageRow[p.Entity], lab.RowMoveRight)
	}
}

func TestTheFloorIsBelowEveryActor(t *testing.T) {
	t.Parallel()
	e := newEngine(t)
	lab.BuildFloor(e)
	if e.Count() != lab.TilesCols*lab.TilesRows {
		t.Fatalf("the floor is %d tiles, want %d", e.Count(), lab.TilesCols*lab.TilesRows)
	}
	for i := range e.Slots() {
		if e.Z[i] >= 0 {
			t.Fatalf("tile %d is on layer %d, want it under layer 0", i, e.Z[i])
		}
	}
}

func TestASecondPressInsideTheCooldownDoesNothing(t *testing.T) {
	t.Parallel()
	e := newEngine(t)
	p := lab.NewPlayer(e)
	b := lab.NewBouncers(e, 0)

	if _, fired := p.Fire(lab.Spawn, b); !fired {
		t.Fatal("the first press did not fire")
	}
	if _, fired := p.Fire(lab.Spawn, b); fired {
		t.Fatal("a press inside the cooldown fired")
	}
	if got := b.Len(); got != lab.SpawnBatch {
		t.Errorf("%d bouncers after one spawn and one refused press, want %d", got, lab.SpawnBatch)
	}
	p.Tick(lab.Spawn.Cooldown() - 1)
	if p.Ready(lab.Spawn) {
		t.Error("ready a millisecond early")
	}
	p.Tick(1)
	if !p.Ready(lab.Spawn) {
		t.Error("not ready when the cooldown ran out")
	}
	if p.Cooldown[lab.Strike] != 0 || !p.Ready(lab.Strike) {
		t.Error("one skill's cooldown reached another")
	}
}

func TestAStrikeTakesOnlyTheBouncersThatOverlap(t *testing.T) {
	t.Parallel()
	e := newEngine(t)
	p := lab.NewPlayer(e)
	b := lab.NewBouncers(e, 0)
	px, py := e.X[p.Entity], e.Y[p.Entity]
	near := []int{b.Add(px+4, py), b.Add(px-8, py+8), b.Add(px, py+12)}
	far := []int{b.Add(px+40, py), b.Add(px, py-100)}

	hit, fired := p.Fire(lab.Strike, b)

	if !fired {
		t.Fatal("the strike did not fire")
	}
	slices.Sort(hit)
	if !slices.Equal(hit, near) {
		t.Errorf("hit = %v, want the near ones %v", hit, near)
	}
	for _, i := range near {
		if e.Live(i) {
			t.Errorf("bouncer %d survived a strike it was inside", i)
		}
	}
	for _, i := range far {
		if !e.Live(i) {
			t.Errorf("bouncer %d was out of reach and went anyway", i)
		}
	}
	if b.Len() != len(far) {
		t.Errorf("%d bouncers left, want %d", b.Len(), len(far))
	}
	// A struck bouncer is gone from the crowd as well as the engine, so the
	// bounce never moves a slot somebody else has taken.
	if vx, vy := b.Velocity(near[0]); vx != 0 || vy != 0 {
		t.Error("a struck bouncer still has a velocity")
	}
}

func TestASpawnPastTheCapAddsNone(t *testing.T) {
	t.Parallel()
	e := newEngine(t)
	p := lab.NewPlayer(e)
	b := lab.NewBouncers(e, 15)

	added, _ := p.Fire(lab.Spawn, b)
	if len(added) != lab.SpawnBatch || b.Len() != lab.SpawnBatch {
		t.Fatalf("the first spawn added %d, want %d", len(added), lab.SpawnBatch)
	}
	p.Tick(lab.Spawn.Cooldown())
	added, _ = p.Fire(lab.Spawn, b)
	if len(added) != 5 || b.Len() != 15 {
		t.Errorf("the spawn at the cap added %d, want the 5 that fit", len(added))
	}
	p.Tick(lab.Spawn.Cooldown())
	added, fired := p.Fire(lab.Spawn, b)
	if !fired || len(added) != 0 || b.Len() != 15 {
		t.Errorf("a spawn past the cap: fired %v, added %d, %d bouncers", fired, len(added), b.Len())
	}
	if e.Count() != 16 {
		t.Errorf("the engine holds %d entities, want the player and 15 bouncers", e.Count())
	}
}

func TestADashEndsOnItsOwnTick(t *testing.T) {
	t.Parallel()
	e := newEngine(t)
	p := lab.NewPlayer(e)
	b := lab.NewBouncers(e, 0)

	// How far a plain tick carries the player, for comparison.
	e.Move(p.Entity, 1, 0)
	before := e.X[p.Entity]
	e.Tick(16)
	plain := e.X[p.Entity] - before

	if _, fired := p.Fire(lab.Dash, b); !fired {
		t.Fatal("the dash did not fire")
	}
	before = e.X[p.Entity]
	e.Tick(16)
	if got := e.X[p.Entity] - before; got != plain*lab.DashFactor {
		t.Errorf("a dashing tick moved %v, want %v times the plain %v", got, lab.DashFactor, plain)
	}

	p.Tick(lab.DashDuration - 1)
	if e.SpeedFactor[p.Entity] != lab.DashFactor {
		t.Error("the dash ended a millisecond early")
	}
	p.Tick(1)
	if e.SpeedFactor[p.Entity] != 1 {
		t.Errorf("SpeedFactor = %v after the dash, want 1", e.SpeedFactor[p.Entity])
	}
	before = e.X[p.Entity]
	e.Tick(16)
	if got := e.X[p.Entity] - before; got != plain {
		t.Errorf("a tick after the dash moved %v, want the plain %v", got, plain)
	}
}

func TestTheBounceTurnsAroundAtTheEdge(t *testing.T) {
	t.Parallel()
	e := newEngine(t)
	b := lab.NewBouncers(e, 0)
	i := b.Add(lab.WorldW-2, lab.WorldH/2)
	b.SetVelocity(i, 1, 0)

	for range 3 {
		b.Simulate(1)
	}
	if e.X[i] <= lab.WorldW {
		t.Fatalf("x = %v after three steps, want it just past the edge", e.X[i])
	}
	if vx, _ := b.Velocity(i); vx != -1 {
		t.Errorf("vx = %v past the edge, want it turned around", vx)
	}
	b.Simulate(1)
	if e.X[i] > lab.WorldW {
		t.Errorf("x = %v a step after turning, want it coming back", e.X[i])
	}
	if e.Playing(i) != lab.TagRun {
		t.Errorf("a bouncer plays %q, want %q", e.Playing(i), lab.TagRun)
	}
}

func TestRemoveLastTakesTheNewest(t *testing.T) {
	t.Parallel()
	e := newEngine(t)
	b := lab.NewBouncers(e, 0)
	first := b.Add(10, 10)
	second := b.Add(20, 20)
	if got := b.RemoveLast(); got != second {
		t.Errorf("RemoveLast = %d, want the newest %d", got, second)
	}
	if !e.Live(first) || e.Live(second) {
		t.Error("the wrong bouncer went")
	}
	if !b.Remove(first) || b.Remove(first) || b.Len() != 0 || b.RemoveLast() != -1 {
		t.Error("Remove and RemoveLast disagree about an empty crowd")
	}
}

func TestClampToWorld(t *testing.T) {
	t.Parallel()
	e := newEngine(t)
	p := lab.NewPlayer(e)
	e.X[p.Entity], e.Y[p.Entity] = -100, 5000
	p.Tick(16)
	if e.X[p.Entity] != lab.TileSize/2 || e.Y[p.Entity] != lab.WorldH-lab.TileSize/2 {
		t.Errorf("clamped to (%v, %v), want the sprite just inside the edge", e.X[p.Entity], e.Y[p.Entity])
	}
}

func TestSkillsAreNamedAndKeyed(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for s := range lab.NumSkills {
		if s.Key() == "" || s.String() == "" || s.Cooldown() <= 0 {
			t.Errorf("skill %d has key %q, name %q, cooldown %v", s, s.Key(), s.String(), s.Cooldown())
		}
		if seen[s.Key()] {
			t.Errorf("two skills share the key %q", s.Key())
		}
		seen[s.Key()] = true
	}
	if lab.NumSkills.Key() != "" || lab.NumSkills.Cooldown() != 0 {
		t.Error("the count is not a skill")
	}
}
