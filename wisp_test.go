package wisp_test

import (
	"math"
	"testing"

	"github.com/andygeiss/wisp-engine"
)

// newEngine returns a fresh engine. Every test gets its own, which is what
// lets them all run in parallel — the engine used to be package-level state,
// and the old suite had to run one test at a time.
func newEngine(t *testing.T) *wisp.Engine {
	t.Helper()
	t.Parallel()
	e := wisp.New(wisp.Config{})
	// The tests step by 100 ms, which is one animation frame. The default cap
	// is 50, so raise it rather than halve every expected distance.
	e.Time.MaxStep = 1000
	return e
}

// entityCount returns the entity count and fails if any array disagrees.
func entityCount(t *testing.T, e *wisp.Engine) int {
	t.Helper()
	n := len(e.State)
	for name, got := range map[string]int{
		"Alpha":        len(e.Alpha),
		"FrameOffset":  len(e.FrameOffset),
		"FrameTime":    len(e.FrameTime),
		"ImageColumn":  len(e.ImageColumn),
		"ImageIndex":   len(e.ImageIndex),
		"ImageRow":     len(e.ImageRow),
		"ScreenSpace":  len(e.ScreenSpace),
		"SpeedFactor":  len(e.SpeedFactor),
		"SpriteHeight": len(e.SpriteHeight),
		"SpriteWidth":  len(e.SpriteWidth),
		"X":            len(e.X),
		"Y":            len(e.Y),
		"Z":            len(e.Z),
	} {
		if got != n {
			t.Errorf("len(%s) = %d, want %d", name, got, n)
		}
	}
	if got := e.Count(); got != n {
		t.Errorf("Count() = %d, want %d", got, n)
	}
	return n
}

func TestAdd(t *testing.T) {
	e := newEngine(t)
	a := e.Add(wisp.Sprite{
		Alpha: 0.5, Column: 1, Height: 16, Image: 4, Row: 2,
		State: wisp.StateVisible, Width: 32, X: 100, Y: 200, Z: 3,
	})
	b := e.Add(wisp.Sprite{
		Height: 8, ScreenSpace: true, State: wisp.StateVisible, Width: 8, X: 1, Y: 1,
	})

	if a != 0 || b != 1 {
		t.Fatalf("indices = %d, %d, want 0, 1", a, b)
	}
	if n := entityCount(t, e); n != 2 {
		t.Fatalf("entities = %d, want 2", n)
	}
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"image index", e.ImageIndex[a], 4},
		{"image column", e.ImageColumn[a], 1},
		{"image row", e.ImageRow[a], 2},
		{"width", e.SpriteWidth[a], 32.0},
		{"height", e.SpriteHeight[a], 16.0},
		{"x", e.X[a], 100.0},
		{"y", e.Y[a], 200.0},
		{"alpha", e.Alpha[a], 0.5},
		{"z", e.Z[a], 3},
		{"speed factor", e.SpeedFactor[a], 1.0},
		{"world space", e.ScreenSpace[a], false},
		{"screen space", e.ScreenSpace[b], true},
		{"an unset alpha is solid, not invisible", e.Alpha[b], 1.0},
		{"an unset speed factor is 1, not still", e.SpeedFactor[b], 1.0},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestDelete(t *testing.T) {
	e := newEngine(t)
	// Z runs backwards so the sorted draw order is not the index order.
	for i, x := range []float64{10, 20, 30} {
		e.Add(wisp.Sprite{Height: 32, State: wisp.StateVisible, Width: 32, X: x, Z: 2 - i})
	}
	e.CamTarget, e.InputTarget = 2, 1

	e.Delete(1)

	if n := entityCount(t, e); n != 2 {
		t.Fatalf("entities = %d, want 2", n)
	}
	if e.X[0] != 10 || e.X[1] != 30 {
		t.Errorf("X = %v, want [10 30]", e.X)
	}
	if e.CamTarget != 1 {
		t.Errorf("CamTarget = %d, want 1 (shifted down)", e.CamTarget)
	}
	if e.InputTarget != -1 {
		t.Errorf("InputTarget = %d, want -1 (it was deleted)", e.InputTarget)
	}

	e.CamTarget = 0
	e.Delete(0)
	if e.CamTarget != -1 {
		t.Errorf("CamTarget = %d, want -1 after deleting its entity", e.CamTarget)
	}
	if n := entityCount(t, e); n != 1 || e.X[0] != 30 {
		t.Errorf("entities = %d, X = %v, want 1 entity at 30", n, e.X)
	}
}

// TestBoundingBox pins the fix, not the bug. The old formula added the margin
// to the left and top edge and then measured the full width from there, so a
// 32 px sprite got a 20x20 box pushed 6 px down and right. These numbers are
// centred.
func TestBoundingBox(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		margin       float64
		wantL, wantT float64
		wantR, wantB float64
	}{
		{"no margin is the whole sprite", 0, 84, 84, 116, 116},
		{"the default keeps a 20x20 box, centred", 6, 90, 90, 110, 110},
		{"a big margin shrinks it, still centred", 12, 96, 96, 104, 104},
		{"a margin past half the sprite is an empty box", 20, 100, 100, 100, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEngine(t)
			e.World.HitBoxMargin = tt.margin
			i := e.Add(wisp.Sprite{Height: 32, Width: 32, X: 100, Y: 100})

			l, top, r, b := e.BoundingBox(i)

			if l != tt.wantL || top != tt.wantT || r != tt.wantR || b != tt.wantB {
				t.Errorf("BoundingBox = %v %v %v %v, want %v %v %v %v",
					l, top, r, b, tt.wantL, tt.wantT, tt.wantR, tt.wantB)
			}
			if cx, cy := (l+r)/2, (top+b)/2; cx != 100 || cy != 100 {
				t.Errorf("box centre = (%v, %v), want the sprite's own (100, 100)", cx, cy)
			}
		})
	}
}

func TestHasCollision(t *testing.T) {
	e := newEngine(t)
	a := e.Add(wisp.Sprite{Height: 32, Width: 32, X: 100, Y: 100})
	near := e.Add(wisp.Sprite{Height: 32, Width: 32, X: 110, Y: 100})
	far := e.Add(wisp.Sprite{Height: 32, Width: 32, X: 200, Y: 100})

	if !e.HasCollision(a, near) || !e.HasCollision(near, a) {
		t.Error("overlapping entities do not collide")
	}
	if e.HasCollision(a, far) {
		t.Error("entities 100 px apart collide")
	}
}

func TestAddTilemap(t *testing.T) {
	t.Parallel()
	t.Run("skips empty and out-of-range tiles", func(t *testing.T) {
		e := newEngine(t)
		e.AddTilemap(wisp.Tilemap{
			Cols: 2, Height: 32, Image: 7, Rows: 2,
			Tiles: []int{0, -1, 99, 4}, TilesetCols: 3, TilesetRows: 5, Width: 32,
		})

		if n := entityCount(t, e); n != 2 {
			t.Fatalf("entities = %d, want 2", n)
		}
		if e.X[0] != 16 || e.Y[0] != 16 || e.ImageColumn[0] != 0 || e.ImageRow[0] != 0 {
			t.Errorf("tile 0 at (%v,%v) col %d row %d, want (16,16) col 0 row 0",
				e.X[0], e.Y[0], e.ImageColumn[0], e.ImageRow[0])
		}
		if e.X[1] != 48 || e.Y[1] != 48 || e.ImageColumn[1] != 1 || e.ImageRow[1] != 1 {
			t.Errorf("tile 4 at (%v,%v) col %d row %d, want (48,48) col 1 row 1",
				e.X[1], e.Y[1], e.ImageColumn[1], e.ImageRow[1])
		}
		if e.ImageIndex[0] != 7 || e.Z[0] != 0 || e.State[0] != wisp.StateVisible {
			t.Errorf("tile image %d z %d state %b, want 7 0 visible",
				e.ImageIndex[0], e.Z[0], e.State[0])
		}
	})

	t.Run("a short array fills what it can", func(t *testing.T) {
		e := newEngine(t)
		e.AddTilemap(wisp.Tilemap{
			Cols: 3, Height: 32, Rows: 3,
			Tiles: []int{1}, TilesetCols: 3, TilesetRows: 5, Width: 32,
		})
		if n := entityCount(t, e); n != 1 {
			t.Errorf("entities = %d, want 1", n)
		}
	})
}

func TestStepCamera(t *testing.T) {
	t.Parallel()
	const w, h = 640.0, 360.0

	t.Run("following", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name           string
			worldW, worldH float64
			x, y           float64
			wantX, wantY   float64
		}{
			{"clamps at the top-left corner", 2000, 2000, 10, 10, 0, 0},
			{"clamps at the bottom-right corner", 2000, 2000, 1990, 1990, 2000 - w, 2000 - h},
			{"keeps the target centred inside", 2000, 2000, 1000, 1000, 1000 - w/2, 1000 - h/2},
			{"centres a world smaller than the view", 320, 180, 50, 50, -160, -90},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				e := newEngine(t)
				e.SetWorldSize(tt.worldW, tt.worldH)
				e.CamTarget = e.Add(wisp.Sprite{
					Height: 32, State: wisp.StateVisible, Width: 32, X: tt.x, Y: tt.y,
				})

				e.Step(16)

				if e.CamX != tt.wantX || e.CamY != tt.wantY {
					t.Errorf("camera = (%v, %v), want (%v, %v)", e.CamX, e.CamY, tt.wantX, tt.wantY)
				}
			})
		}
	})

	t.Run("smoothing lags behind the target", func(t *testing.T) {
		e := newEngine(t)
		e.Camera.Smoothing = 8
		e.SetWorldSize(2000, 2000)
		e.CamTarget = e.Add(wisp.Sprite{
			Height: 32, State: wisp.StateVisible, Width: 32, X: 1000, Y: 1000,
		})

		e.Step(16)

		want := 1000 - w/2
		if e.CamX <= 0 || e.CamX >= want {
			t.Errorf("CamX = %v, want it on the way to %v but not there yet", e.CamX, want)
		}
		for range 300 {
			e.Step(16)
		}
		if math.Abs(e.CamX-want) > 1 {
			t.Errorf("CamX = %v after five seconds, want it to have arrived at %v", e.CamX, want)
		}
	})

	t.Run("a target past the end is ignored", func(t *testing.T) {
		e := newEngine(t)
		e.CamTarget = 5
		e.Step(16)
	})

	t.Run("a shake fades out and stops", func(t *testing.T) {
		e := newEngine(t)
		e.Shake(5, 100)

		e.Step(16)
		if !e.Shaking() {
			t.Error("the shake stopped on its first frame")
		}
		if math.Abs(e.CamShakeX) > 5 || math.Abs(e.CamShakeY) > 5 {
			t.Errorf("shake = (%v, %v), want both within the magnitude of 5", e.CamShakeX, e.CamShakeY)
		}

		// The frame that runs the timer out is the last one that shakes.
		e.Step(100)
		if e.Shaking() {
			t.Error("the shake is still running 116 ms into a 100 ms shake")
		}
		if e.CamShakeX != 0 || e.CamShakeY != 0 {
			t.Errorf("shake = (%v, %v) after it ran out, want (0, 0)", e.CamShakeX, e.CamShakeY)
		}
	})

	t.Run("a hit stop freezes the world but not the shake", func(t *testing.T) {
		e := newEngine(t)
		e.CamTarget = e.Add(wisp.Sprite{
			Height: 32, State: wisp.StateVisible | wisp.StateMoveRight, Width: 32, X: 100, Y: 100,
		})
		e.Impact(e.Feel.Heavy)
		x := e.X[0]

		e.Step(16)

		if e.X[0] != x {
			t.Errorf("x = %v, want %v: the world moved during a hit stop", e.X[0], x)
		}
		if !e.Shaking() {
			t.Error("the shake stopped during a hit stop, so the freeze has nothing to feel")
		}
	})

	t.Run("a shake that respects the hit stop freezes with the world", func(t *testing.T) {
		e := newEngine(t)
		e.Feel.Shake.IgnoresHitStop = false
		e.Shake(5, 100)
		e.HitStop(1000, 0)

		for range 10 {
			e.Step(16)
		}

		if !e.Shaking() {
			t.Error("the shake ran out while the world was frozen, so it did not freeze with it")
		}
	})
}

func TestStepMovement(t *testing.T) {
	t.Parallel()
	const attack = uint64(1 << 20)

	// One 100 ms frame at speed factor 1.
	step := func(e *wisp.Engine) float64 { return e.World.Speed * 100 }

	t.Run("no entities and no target does not panic", func(t *testing.T) {
		e := newEngine(t)
		e.Step(16)
		e.InputTarget = 0
		e.Step(16)
	})

	t.Run("the movement keys move the target and pick the row", func(t *testing.T) {
		e := newEngine(t)
		e.RowMask = wisp.MaskPose
		e.RowForState = map[uint64]int{
			wisp.StateFaceRight | wisp.StateIdle: 0,
			wisp.StateFaceLeft | wisp.StateIdle:  1,
			wisp.StateFaceRight | wisp.StateMove: 2,
			wisp.StateFaceLeft | wisp.StateMove:  3,
		}
		p := e.Add(wisp.Sprite{
			Height: 32, State: wisp.StateFaceRight | wisp.StateIdle | wisp.StateVisible,
			Width: 32, X: 100, Y: 100,
		})
		e.InputTarget = p
		e.FrameOffset[p] = 5

		e.Input.Key("a", true)
		e.Step(100)

		if e.X[p] != 100-step(e) {
			t.Errorf("x = %v, want %v", e.X[p], 100-step(e))
		}
		s := e.State[p]
		if s&wisp.StateMoveLeft == 0 || s&wisp.StateFaceLeft == 0 || s&wisp.StateMove == 0 {
			t.Errorf("state %b lacks MoveLeft, FaceLeft, or Move", s)
		}
		if s&wisp.StateFaceRight != 0 || s&wisp.StateIdle != 0 {
			t.Errorf("state %b still has FaceRight or Idle", s)
		}
		if e.ImageRow[p] != 3 || e.FrameOffset[p] != 0 {
			t.Errorf("row %d frame %d, want row 3 and the animation restarted", e.ImageRow[p], e.FrameOffset[p])
		}

		e.Input.Key("a", false)
		e.Step(100)

		if e.X[p] != 100-step(e) {
			t.Errorf("x = %v after release, want %v", e.X[p], 100-step(e))
		}
		s = e.State[p]
		if s&wisp.StateIdle == 0 || s&wisp.StateMove != 0 || s&wisp.StateFaceLeft == 0 {
			t.Errorf("state %b, want Idle and FaceLeft without Move", s)
		}
		if e.ImageRow[p] != 1 {
			t.Errorf("row = %d, want 1", e.ImageRow[p])
		}
	})

	t.Run("the arrow keys drive it too", func(t *testing.T) {
		e := newEngine(t)
		p := e.Add(wisp.Sprite{Height: 32, State: wisp.StateVisible, Width: 32, X: 100, Y: 100})
		e.InputTarget = p

		e.Input.Key("ArrowRight", true)
		e.Step(100)

		if e.X[p] != 100+step(e) {
			t.Errorf("x = %v, want %v", e.X[p], 100+step(e))
		}
	})

	t.Run("a diagonal is as fast as a straight line", func(t *testing.T) {
		e := newEngine(t)
		p := e.Add(wisp.Sprite{Height: 32, State: wisp.StateVisible, Width: 32, X: 100, Y: 100})
		e.InputTarget = p
		e.Input.Key("a", true)
		e.Input.Key("w", true)

		e.Step(100)

		want := step(e) / math.Sqrt2
		if math.Abs(e.X[p]-(100-want)) > 1e-9 || math.Abs(e.Y[p]-(100-want)) > 1e-9 {
			t.Errorf("moved to (%v, %v), want (%v, %v)", e.X[p], e.Y[p], 100-want, 100-want)
		}
	})

	t.Run("the speed factor scales the step", func(t *testing.T) {
		e := newEngine(t)
		p := e.Add(wisp.Sprite{
			Height: 32, SpeedFactor: 2, State: wisp.StateVisible | wisp.StateMoveRight,
			Width: 32, X: 100, Y: 100,
		})

		e.Step(100)

		if e.X[p] != 100+2*step(e) {
			t.Errorf("x = %v, want %v", e.X[p], 100+2*step(e))
		}
	})

	t.Run("an action locks the facing and picks its own row", func(t *testing.T) {
		e := newEngine(t)
		e.RowMask = wisp.MaskPose | attack
		e.RowForState = map[uint64]int{attack | wisp.StateFaceRight: 9}
		p := e.Add(wisp.Sprite{
			Height: 32, State: wisp.StateFaceRight | attack | wisp.StateVisible,
			Width: 32, X: 100, Y: 100,
		})
		e.InputTarget = p
		e.Input.Key("a", true)

		e.Step(100)

		s := e.State[p]
		if s&wisp.StateFaceRight == 0 || s&wisp.StateFaceLeft != 0 {
			t.Errorf("state %b turned around during an action", s)
		}
		if s&wisp.StateMoveLeft == 0 {
			t.Errorf("state %b did not move", s)
		}
		if e.ImageRow[p] != 9 {
			t.Errorf("row = %d, want 9 (the action row, idle and move dropped from the key)", e.ImageRow[p])
		}
	})

	t.Run("static entities are left alone", func(t *testing.T) {
		e := newEngine(t)
		e.RowMask = wisp.MaskPose
		tile := e.Add(wisp.Sprite{Height: 32, State: wisp.StateVisible, Width: 32, X: 16, Y: 16})

		e.Step(100)

		if e.State[tile] != wisp.StateVisible || e.X[tile] != 16 {
			t.Errorf("tile state %b at x %v, want untouched", e.State[tile], e.X[tile])
		}
	})

	t.Run("move bits move an entity that is not the input target", func(t *testing.T) {
		e := newEngine(t)
		p := e.Add(wisp.Sprite{
			Height: 32, State: wisp.StateVisible | wisp.StateMoveRight, Width: 32, X: 100, Y: 100,
		})

		e.Step(100)

		if e.X[p] != 100+step(e) {
			t.Errorf("x = %v, want %v", e.X[p], 100+step(e))
		}
	})

	t.Run("a pause holds everything still", func(t *testing.T) {
		e := newEngine(t)
		p := e.Add(wisp.Sprite{
			Height: 32, State: wisp.StateVisible | wisp.StateMoveRight, Width: 32, X: 100, Y: 100,
		})
		e.Paused = true

		e.Step(100)

		if e.X[p] != 100 {
			t.Errorf("x = %v, want 100: a paused world moved", e.X[p])
		}
	})

	t.Run("the time scale slows the world down", func(t *testing.T) {
		e := newEngine(t)
		e.Time.Scale = 0.5
		p := e.Add(wisp.Sprite{
			Height: 32, State: wisp.StateVisible | wisp.StateMoveRight, Width: 32, X: 100, Y: 100,
		})

		e.Step(100)

		if e.X[p] != 100+step(e)/2 {
			t.Errorf("x = %v, want %v at half speed", e.X[p], 100+step(e)/2)
		}
	})

	t.Run("the frame cap stops a background tab teleporting things", func(t *testing.T) {
		e := newEngine(t)
		e.Time.MaxStep = 50
		p := e.Add(wisp.Sprite{
			Height: 32, State: wisp.StateVisible | wisp.StateMoveRight, Width: 32, X: 100, Y: 100,
		})

		e.Step(10000)

		if want := 100 + e.World.Speed*50; e.X[p] != want {
			t.Errorf("x = %v after a ten-second frame, want %v", e.X[p], want)
		}
	})
}

func TestStepAnimation(t *testing.T) {
	t.Parallel()

	t.Run("a one-shot animation hides its entity at the end", func(t *testing.T) {
		e := newEngine(t)
		i := e.Add(wisp.Sprite{
			Height: 32, State: wisp.StateAnimated | wisp.StateAutoHide | wisp.StateVisible,
			Width: 32,
		})

		for range e.Animation.FrameCount - 1 {
			e.Step(e.Animation.FrameDuration)
		}
		if e.FrameOffset[i] != e.Animation.FrameCount-1 || e.State[i]&wisp.StateVisible == 0 {
			t.Fatalf("before the last frame: offset %d state %b, want offset %d and visible",
				e.FrameOffset[i], e.State[i], e.Animation.FrameCount-1)
		}

		e.Step(e.Animation.FrameDuration)
		if e.FrameOffset[i] != 0 || e.State[i]&(wisp.StateAnimated|wisp.StateVisible) != 0 {
			t.Errorf("after the last frame: offset %d state %b, want offset 0, not animated, hidden",
				e.FrameOffset[i], e.State[i])
		}
	})

	t.Run("a looping animation starts over", func(t *testing.T) {
		e := newEngine(t)
		i := e.Add(wisp.Sprite{
			Height: 32, State: wisp.StateAnimated | wisp.StateAnimatedLoop | wisp.StateVisible,
			Width: 32,
		})

		for range e.Animation.FrameCount {
			e.Step(e.Animation.FrameDuration)
		}
		if e.FrameOffset[i] != 0 || e.State[i]&wisp.StateAnimated == 0 || e.State[i]&wisp.StateVisible == 0 {
			t.Errorf("offset %d state %b, want offset 0, animated, visible", e.FrameOffset[i], e.State[i])
		}
	})

	t.Run("frame time accumulates across frames", func(t *testing.T) {
		e := newEngine(t)
		i := e.Add(wisp.Sprite{Height: 32, State: wisp.StateAnimated, Width: 32})

		e.Step(60)
		if e.FrameOffset[i] != 0 || e.FrameTime[i] != 60 {
			t.Errorf("after 60 ms: offset %d time %v, want 0 and 60", e.FrameOffset[i], e.FrameTime[i])
		}
		e.Step(60)
		if e.FrameOffset[i] != 1 || e.FrameTime[i] != 0 {
			t.Errorf("after 120 ms: offset %d time %v, want 1 and 0", e.FrameOffset[i], e.FrameTime[i])
		}
	})

	t.Run("a still entity stays on its frame", func(t *testing.T) {
		e := newEngine(t)
		i := e.Add(wisp.Sprite{Height: 32, State: wisp.StateVisible, Width: 32})

		e.Step(500)

		if e.FrameOffset[i] != 0 || e.FrameTime[i] != 0 {
			t.Errorf("offset %d time %v, want both 0", e.FrameOffset[i], e.FrameTime[i])
		}
	})

	// The animation used to advance inside the draw pass, behind the viewport
	// cull, so an entity outside the view always froze. Now it is a knob.
	t.Run("off-screen entities animate unless you ask them not to", func(t *testing.T) {
		t.Parallel()
		for _, cull := range []bool{false, true} {
			t.Run(map[bool]string{false: "cull off", true: "cull on"}[cull], func(t *testing.T) {
				e := newEngine(t)
				e.Animation.CullOffscreen = cull
				e.SetWorldSize(10000, 10000)
				near := e.Add(wisp.Sprite{
					Height: 32, State: wisp.StateAnimated | wisp.StateAnimatedLoop | wisp.StateVisible,
					Width: 32, X: 100, Y: 100,
				})
				far := e.Add(wisp.Sprite{
					Height: 32, State: wisp.StateAnimated | wisp.StateAnimatedLoop | wisp.StateVisible,
					Width: 32, X: 5000, Y: 5000,
				})

				e.Step(e.Animation.FrameDuration)

				if e.FrameOffset[near] != 1 {
					t.Errorf("the entity in view is on frame %d, want 1", e.FrameOffset[near])
				}
				wantFar := 1
				if cull {
					wantFar = 0
				}
				if e.FrameOffset[far] != wantFar {
					t.Errorf("the entity out of view is on frame %d, want %d",
						e.FrameOffset[far], wantFar)
				}
			})
		}
	})
}

func TestInput(t *testing.T) {
	t.Parallel()

	t.Run("a key is held until it is released", func(t *testing.T) {
		e := newEngine(t)
		e.Input.Key("q", true)

		if !e.Input.Down("q") {
			t.Error("q is not down after pressing it")
		}
		e.Step(16)
		if !e.Input.Down("q") {
			t.Error("q stopped being down after a frame, without being released")
		}
		e.Input.Key("q", false)
		if e.Input.Down("q") {
			t.Error("q is still down after releasing it")
		}
	})

	t.Run("an edge lasts exactly one frame", func(t *testing.T) {
		e := newEngine(t)
		e.Input.Key("n", true)

		if !e.Input.JustPressed("n") {
			t.Error("pressing n did not report a press")
		}
		e.Step(16)
		if e.Input.JustPressed("n") {
			t.Error("the press is still reported a frame later, so a game must still clear it by hand")
		}
	})

	t.Run("a tap inside one frame reports both edges", func(t *testing.T) {
		e := newEngine(t)
		e.Input.Key("e", true)
		e.Input.Key("e", false)

		if !e.Input.JustPressed("e") || !e.Input.JustReleased("e") {
			t.Error("a press and release inside one frame lost an edge")
		}
		if e.Input.Down("e") {
			t.Error("e is down after being released")
		}
	})

	t.Run("a letter is one key whatever the shift state", func(t *testing.T) {
		e := newEngine(t)
		e.Input.Key("W", true)

		if !e.Input.Down("w") {
			t.Error(`"W" and "w" are not the same key`)
		}
	})

	t.Run("a named key keeps its name", func(t *testing.T) {
		e := newEngine(t)
		e.Input.Key("ArrowLeft", true)

		if !e.Input.Down("ArrowLeft") {
			t.Error("ArrowLeft is not down after pressing it")
		}
		if e.Input.Down("arrowleft") {
			t.Error("a named key was folded to lower case")
		}
	})

	t.Run("the first key marks the player as present", func(t *testing.T) {
		e := newEngine(t)
		if e.Input.Started {
			t.Error("the player is present before touching anything")
		}
		e.Input.Key("z", true)
		if !e.Input.Started {
			t.Error("pressing a key did not mark the player as present")
		}
	})
}

// press taps a key and runs one frame, which is how a person uses the menu:
// one press, one frame.
func press(e *wisp.Engine, key string) {
	e.Input.Key(key, true)
	e.Step(16)
	e.Input.Key(key, false)
}

// openHeavy walks the menu to Feel.Heavy's first knob. The groups are
// alphabetical, so Feel.Heavy is the fifth heading — Animation, Audio, Camera,
// Debug, then it.
func openHeavy(e *wisp.Engine) {
	for range 4 {
		press(e, "ArrowDown")
	}
	press(e, "Enter")     // open Feel.Heavy
	press(e, "ArrowDown") // its first knob, HitStopDuration
}

func TestMenu(t *testing.T) {
	t.Parallel()

	t.Run("M opens it and takes the keyboard", func(t *testing.T) {
		e := newEngine(t)
		if e.Menu() {
			t.Fatal("the menu is open before anybody asked for it")
		}

		press(e, "m")

		if !e.Menu() {
			t.Fatal("M did not open the menu")
		}
		// A game must not be steerable underneath an open menu.
		e.Input.Key("a", true)
		if e.Input.Down("a") || e.Input.JustPressed("a") {
			t.Error("the game can still read the keyboard while the menu is open")
		}

		e.Input.Key("a", false)
		press(e, "m")
		if e.Menu() {
			t.Error("M did not close the menu")
		}
		e.Input.Key("a", true)
		if !e.Input.Down("a") {
			t.Error("the keyboard is still locked after the menu closed")
		}
	})

	t.Run("a game can turn the menu off", func(t *testing.T) {
		t.Parallel()
		e := wisp.New(wisp.Config{MenuKey: wisp.KeyNone})
		press(e, "m")
		if e.Menu() {
			t.Error("the menu opened in a game that asked for none")
		}
	})

	t.Run("the arrows nudge a knob by its own step", func(t *testing.T) {
		e := newEngine(t)
		press(e, "m")
		openHeavy(e)

		before := e.Feel.Heavy.HitStopDuration
		press(e, "ArrowRight")
		if got := e.Feel.Heavy.HitStopDuration; got != before+5 {
			t.Fatalf("HitStopDuration = %v, want %v after one step", got, before+5)
		}

		e.Input.SetModifiers(false, false, false, true)
		press(e, "ArrowRight")
		e.Input.SetModifiers(false, false, false, false)
		if got := e.Feel.Heavy.HitStopDuration; got != before+55 {
			t.Errorf("HitStopDuration = %v, want %v after a shifted step", got, before+55)
		}

		press(e, "Backspace")
		if got := e.Feel.Heavy.HitStopDuration; got != before {
			t.Errorf("HitStopDuration = %v, want %v after a reset", got, before)
		}
	})

	t.Run("a knob cannot be nudged out of its range", func(t *testing.T) {
		e := newEngine(t)
		press(e, "m")
		openHeavy(e)

		for range 200 {
			press(e, "ArrowLeft")
		}
		if got := e.Feel.Heavy.HitStopDuration; got != 0 {
			t.Errorf("HitStopDuration = %v, want it stopped at its minimum of 0", got)
		}
	})

	t.Run("shift and backspace resets everything", func(t *testing.T) {
		e := newEngine(t)
		e.Feel.Heavy.ShakeMagnitude = 31
		e.World.Speed = 0.9
		press(e, "m")

		e.Input.SetModifiers(false, false, false, true)
		press(e, "Backspace")
		e.Input.SetModifiers(false, false, false, false)

		def := wisp.Defaults()
		if e.Feel.Heavy.ShakeMagnitude != def.Feel.Heavy.ShakeMagnitude || e.World.Speed != def.World.Speed {
			t.Errorf("shake %v speed %v, want the defaults %v and %v",
				e.Feel.Heavy.ShakeMagnitude, e.World.Speed,
				def.Feel.Heavy.ShakeMagnitude, def.World.Speed)
		}
	})

	t.Run("escape closes it", func(t *testing.T) {
		e := newEngine(t)
		press(e, "m")
		press(e, "Escape")
		if e.Menu() {
			t.Error("escape did not close the menu")
		}
	})
}
