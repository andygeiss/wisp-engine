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

func TestTheSheetNamesTheTags(t *testing.T) {
	t.Parallel()
	s := lab.HeroSheet()
	if got := s.Lookup("walk-east"); got != lab.Tag(lab.HeroWalk, lab.East) {
		t.Errorf("Lookup(walk-east) = %d, want %d", got, lab.Tag(lab.HeroWalk, lab.East))
	}
	if got := s.Lookup("face-south-west"); got != lab.Tag(lab.HeroFace, lab.SouthWest) {
		t.Errorf("Lookup(face-south-west) = %d, want %d", got, lab.Tag(lab.HeroFace, lab.SouthWest))
	}
	rows := 1 + lab.NumDirections*(lab.NumHeroAnims-1)
	if len(s.Tags) != lab.NumDirections*lab.NumHeroAnims || len(s.Frames) != rows*lab.Columns {
		t.Errorf("the hero's sheet has %d tags and %d frames", len(s.Tags), len(s.Frames))
	}
	// Every tag stays inside its own row, and the face tags share row 0.
	for i, tag := range s.Tags {
		if tag.From/lab.Columns != tag.To/lab.Columns {
			t.Errorf("tag %d %q spans rows: %d..%d", i, tag.Name, tag.From, tag.To)
		}
		if i < lab.NumDirections && tag.From != i {
			t.Errorf("face tag %d starts at frame %d, want column %d of row 0", i, tag.From, i)
		}
	}

	// A player walking right is drawn from walk-east; up and left from
	// walk-north-west; standing after that from idle-north-west.
	e := newEngine(t)
	p := lab.NewPlayer(e)
	e.Move(p.Entity, 1, 0)
	e.Tick(16)
	if e.ImageRow[p.Entity] != lab.Tag(lab.HeroWalk, lab.East) {
		t.Errorf("walking right is on tag %d, want %d", e.ImageRow[p.Entity], lab.Tag(lab.HeroWalk, lab.East))
	}
	e.Move(p.Entity, -1, -1)
	e.Tick(16)
	if e.ImageRow[p.Entity] != lab.Tag(lab.HeroWalk, lab.NorthWest) {
		t.Errorf("walking up and left is on tag %d, want %d", e.ImageRow[p.Entity], lab.Tag(lab.HeroWalk, lab.NorthWest))
	}
	e.Move(p.Entity, 0, 0)
	e.Tick(16)
	if e.ImageRow[p.Entity] != lab.Tag(lab.HeroIdle, lab.NorthWest) {
		t.Errorf("standing is on tag %d, want %d", e.ImageRow[p.Entity], lab.Tag(lab.HeroIdle, lab.NorthWest))
	}
}

func TestDirectionOf(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		vx, vy float64
		want   int
	}{
		{1, 0, lab.East}, {-1, 0, lab.West}, {0, 1, lab.South}, {0, -1, lab.North},
		{1, 1, lab.SouthEast}, {1, -1, lab.NorthEast}, {-1, 1, lab.SouthWest}, {-1, -1, lab.NorthWest},
		{1, 0.3, lab.East}, {1, 0.5, lab.SouthEast}, {-0.2, -1, lab.North}, {-0.6, -1, lab.NorthWest},
		{0, 0, lab.South},
	} {
		if got := lab.DirectionOf(c.vx, c.vy); got != c.want {
			t.Errorf("DirectionOf(%v, %v) = %s, want %s", c.vx, c.vy, lab.Directions[got], lab.Directions[c.want])
		}
	}
}

func TestTheFloorIsBelowEveryActor(t *testing.T) {
	t.Parallel()
	e := newEngine(t)
	lab.BuildFloor(e)
	path := 0
	for _, tile := range lab.Wang(lab.PathVertices(), lab.TilesCols, lab.TilesRows, lab.WangTable, true) {
		if tile >= 0 {
			path++
		}
	}
	if path == 0 {
		t.Fatal("the path has no tiles")
	}
	if e.Count() != lab.TilesCols*lab.TilesRows+path {
		t.Fatalf("the floor is %d tiles, want %d of meadow and %d of path", e.Count(), lab.TilesCols*lab.TilesRows, path)
	}
	for i := range e.Slots() {
		switch e.ImageIndex[i] {
		case lab.ImageMeadow:
			if e.Z[i] != lab.ZFloor {
				t.Fatalf("meadow tile %d is on layer %d, want %d", i, e.Z[i], lab.ZFloor)
			}
		case lab.ImagePath:
			if e.Z[i] != lab.ZPath {
				t.Fatalf("path tile %d is on layer %d, want %d", i, e.Z[i], lab.ZPath)
			}
		default:
			t.Fatalf("tile %d is drawn from image %d", i, e.ImageIndex[i])
		}
		if e.Z[i] >= 0 {
			t.Fatalf("tile %d is on layer %d, want it under layer 0", i, e.Z[i])
		}
	}
	if lab.ZPath <= lab.ZFloor || lab.ZProps < 0 || lab.ZPlayer < 0 {
		t.Error("the layers are out of order")
	}
}

func TestWangPicksATileByItsCorners(t *testing.T) {
	t.Parallel()
	// Two cells side by side from six vertices: the left one has upper
	// terrain on three corners and lower at the south-east, the right one
	// upper at the north-west only.
	vertices := []uint8{
		1, 1, 0,
		1, 0, 0,
	}
	tiles := lab.Wang(vertices, 2, 1, lab.WangTable, false)
	if want := []int{lab.WangTable[8+4+2], lab.WangTable[8]}; !slices.Equal(tiles, want) {
		t.Errorf("tiles = %v, want %v", tiles, want)
	}
	if got := lab.Wang([]uint8{0, 0, 0, 0}, 1, 1, lab.WangTable, true); got[0] != -1 {
		t.Errorf("a cell of lower terrain on a skipping layer = %d, want empty", got[0])
	}
	if got := lab.Wang([]uint8{0, 0, 0, 0}, 1, 1, lab.WangTable, false); got[0] != lab.WangTable[0] {
		t.Errorf("a cell of lower terrain on the base layer = %d, want tile %d", got[0], lab.WangTable[0])
	}
	// The table is a permutation of the sheet: every tile once.
	seen := make([]bool, 16)
	for idx, tile := range lab.WangTable {
		if tile < 0 || tile >= 16 || seen[tile] {
			t.Errorf("index %d maps to tile %d, which is out of the sheet or taken", idx, tile)
		}
		seen[tile] = true
	}
}

func TestThePathStaysOnTheGrass(t *testing.T) {
	t.Parallel()
	lake, path := lab.LakeVertices(), lab.PathVertices()
	w := lab.TilesCols + 1
	if len(lake) != w*(lab.TilesRows+1) || len(path) != len(lake) {
		t.Fatalf("%d and %d vertices, want %d", len(lake), len(path), w*(lab.TilesRows+1))
	}
	water, grass, dirt := 0, 0, 0
	for y := range lab.TilesRows + 1 {
		for x := range w {
			v := lake[y*w+x]
			switch v {
			case 0:
				water++
			case 1:
				grass++
			}
			if (x == 0 || y == 0 || x == w-1 || y == lab.TilesRows) && v != 1 {
				t.Errorf("vertex (%d, %d) on the world's edge is water; the edge is land", x, y)
			}
			if path[y*w+x] == 1 {
				dirt++
			}
		}
	}
	if water == 0 || grass == 0 || dirt == 0 {
		t.Fatalf("%d water, %d grass, %d dirt vertices: the level lacks a terrain", water, grass, dirt)
	}
	// Wherever the path draws a tile, the meadow under it is all grass, or
	// the path would cover a shore.
	for y := range lab.TilesRows {
		for x := range lab.TilesCols {
			corners := func(v []uint8) int {
				return int(v[y*w+x])<<3 | int(v[y*w+x+1])<<2 | int(v[(y+1)*w+x])<<1 | int(v[(y+1)*w+x+1])
			}
			if corners(path) != 0 && corners(lake) != 15 {
				t.Errorf("cell (%d, %d) has a path tile over a shore", x, y)
			}
		}
	}
}

func TestPropsStandOffTheWaterAndThePath(t *testing.T) {
	t.Parallel()
	lake, path := lab.LakeVertices(), lab.PathVertices()
	w := lab.TilesCols + 1
	// A player spawns at the world's centre, so that cell is grass too.
	cx, cy := lab.TilesCols/2, lab.TilesRows/2
	if lake[cy*w+cx]&lake[cy*w+cx+1]&lake[(cy+1)*w+cx]&lake[(cy+1)*w+cx+1] != 1 {
		t.Error("the world's centre, where a player spawns, is in the water")
	}
	for _, p := range lab.Props {
		if p.Image < 0 || p.Image >= lab.NumImages || p.W <= 0 || p.H <= 0 {
			t.Errorf("prop %+v is not a sprite", p)
		}
		// The cell under the prop's feet, and its corners.
		x, y := int(p.X/lab.TileSize), int((p.Y+p.H/2-1)/lab.TileSize)
		if x < 0 || y < 0 || x >= lab.TilesCols || y >= lab.TilesRows {
			t.Errorf("prop %+v stands outside the world", p)
			continue
		}
		for _, v := range [][]uint8{lake} {
			if v[y*w+x]&v[y*w+x+1]&v[(y+1)*w+x]&v[(y+1)*w+x+1] != 1 {
				t.Errorf("prop %+v stands in the water", p)
			}
		}
		if path[y*w+x]|path[y*w+x+1]|path[(y+1)*w+x]|path[(y+1)*w+x+1] != 0 {
			t.Errorf("prop %+v stands on the path", p)
		}
	}
	e := newEngine(t)
	lab.BuildProps(e)
	if e.Count() != len(lab.Props) {
		t.Errorf("%d props built, want %d", e.Count(), len(lab.Props))
	}
	for i := range e.Slots() {
		if e.Z[i] != lab.ZProps {
			t.Errorf("prop %d is on layer %d, want %d", i, e.Z[i], lab.ZProps)
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
	far := []int{b.Add(px+80, py), b.Add(px, py-100)}

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

func TestAStrikeSwingsForItsAnimation(t *testing.T) {
	t.Parallel()
	e := newEngine(t)
	p := lab.NewPlayer(e)
	b := lab.NewBouncers(e, 0)

	if _, fired := p.Fire(lab.Strike, b); !fired {
		t.Fatal("the strike did not fire")
	}
	if e.State[p.Entity]&lab.StateStrike == 0 {
		t.Fatal("a strike did not set its bit")
	}
	// Mid-swing the player walks left and is still drawn striking south:
	// an action locks the facing.
	e.Move(p.Entity, -1, 0)
	e.Tick(16)
	if s := e.State[p.Entity]; s&wisp.StateFaceDown == 0 || s&wisp.StateFaceLeft != 0 {
		t.Errorf("state %b turned during the swing", s)
	}
	if e.ImageRow[p.Entity] != lab.Tag(lab.HeroStrike, lab.South) {
		t.Errorf("mid-swing on tag %d, want %d", e.ImageRow[p.Entity], lab.Tag(lab.HeroStrike, lab.South))
	}
	// The player's own Tick counts the swing down, the way the server calls
	// it after every engine tick.
	p.Tick(lab.StrikeDuration - 1)
	if e.State[p.Entity]&lab.StateStrike == 0 {
		t.Error("the swing ended a millisecond early")
	}
	p.Tick(1)
	if e.State[p.Entity]&lab.StateStrike != 0 {
		t.Error("the swing did not end with its animation")
	}
	// The server steers a player every tick, so the axis it is still
	// holding turns it the moment the swing is over.
	e.Move(p.Entity, -1, 0)
	e.Tick(16)
	if e.ImageRow[p.Entity] != lab.Tag(lab.HeroWalk, lab.West) {
		t.Errorf("after the swing on tag %d, want walk-west %d", e.ImageRow[p.Entity], lab.Tag(lab.HeroWalk, lab.West))
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
	if e.Playing(i) != "walk-east" {
		t.Errorf("a bouncer launched east plays %q", e.Playing(i))
	}
	e.FrameOffset[i] = 3

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
	if e.Playing(i) != "walk-west" {
		t.Errorf("a bouncer turned around plays %q, want walk-west", e.Playing(i))
	}
	if e.FrameOffset[i] != 3 {
		t.Errorf("frame %d after turning, want the stride kept at 3", e.FrameOffset[i])
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
	if e.X[p.Entity] != lab.HeroW/2 || e.Y[p.Entity] != lab.WorldH-lab.HeroH/2 {
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
