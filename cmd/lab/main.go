//go:build js && wasm

// The lab: one scene that exists to be measured and tuned.
//
// Press M for the tuning menu, then nudge a knob and watch the same hit land
// differently. Press ] until the frame time breaks, and the number you stopped
// at is what this renderer can carry.
package main

import (
	"math/rand/v2"

	"github.com/andygeiss/wisp-engine"
)

// Images, in the order LoadImages receives them.
const (
	imageSprites = iota
	imageTiles
)

// Rows of the sheet, one animation each. They are the game-jam-template's own
// sheet, so the lab animates without anybody drawing anything new.
const (
	rowIdleRight = iota
	rowIdleLeft
	rowMoveRight
	rowMoveLeft
)

const (
	tileSize   = 32.0
	tilesCols  = 40
	tilesRows  = 24
	tilesetCol = 3
	tilesetRow = 5
	worldW     = tilesCols * tileSize
	worldH     = tilesRows * tileSize

	// zFloor is below every layer spawn uses, so the floor never draws over an
	// actor standing on it.
	zFloor = -1

	// spawnBatch is how many sprites one press adds. Shift multiplies it by
	// ten, so walking the count up to the ceiling takes a few seconds rather
	// than a few minutes.
	spawnBatch = 100

	// rampPerSecond is how fast the automatic ramp adds sprites. Slow enough
	// that the frame window has caught up before the next batch lands, which
	// is what makes the number it stops at mean something.
	rampPerSecond = 100
	// rampSettle is how long the ramp waits before it starts believing the
	// frame times, so the first slow frames after a reset do not end it.
	rampSettle = 1000.0
)

// A spawned sprite carries its own velocity: the engine moves entities by
// state bits, which is a game's job to set, and bouncing is this lab's game.
var (
	first  int // the index the spawned sprites start at
	player int
	vx     []float64
	vy     []float64

	// The automatic ramp: it adds sprites until the frame time breaks, then
	// stops and holds the count. That number is what the lab exists to find.
	rampAt      float64
	rampCeiling int
	ramping     bool
	settled     float64
)

func main() {
	e := wisp.New(wisp.Config{})
	e.LoadImages("/static/img/lab.png", "/static/img/tiles.png")
	e.RowMask = wisp.MaskPose
	e.RowForState = map[uint64]int{
		wisp.StateFaceRight | wisp.StateIdle: rowIdleRight,
		wisp.StateFaceLeft | wisp.StateIdle:  rowIdleLeft,
		wisp.StateFaceRight | wisp.StateMove: rowMoveRight,
		wisp.StateFaceLeft | wisp.StateMove:  rowMoveLeft,
	}
	e.SetWorldSize(worldW, worldH)
	e.RenderUI = func() { renderUI(e) }

	build(e)
	e.Run(func(dt float64) { update(e, dt) })
}

// build lays out the scene: a floor, a player, and nothing else. Everything
// after that is spawned by hand, so the count on screen is the count you asked
// for.
func build(e *wisp.Engine) {
	e.Reset()
	vx, vy = vx[:0], vy[:0]
	ramping, rampAt, rampCeiling, settled = false, 0, 0, 0

	tiles := make([]int, tilesCols*tilesRows)
	for i := range tiles {
		tiles[i] = 4 // the floor tile of the template's tileset
	}
	// The floor goes below every actor. Sharing a layer with them would not
	// hide it behind them: inside one layer the sort is by baseline, so each
	// tile below a sprite's middle draws after it and repaints its lower half.
	e.AddTilemap(wisp.Tilemap{
		Cols: tilesCols, Height: tileSize, Image: imageTiles, Rows: tilesRows,
		Tiles: tiles, TilesetCols: tilesetCol, TilesetRows: tilesetRow, Width: tileSize,
		Z: zFloor,
	})

	player = e.Add(wisp.Sprite{
		Height: tileSize, Image: imageSprites,
		State: wisp.StateAnimated | wisp.StateAnimatedLoop | wisp.StateFaceRight |
			wisp.StateIdle | wisp.StateVisible,
		Width: tileSize, X: worldW / 2, Y: worldH / 2, Z: 1,
	})
	e.CamTarget, e.InputTarget = player, player
	first = e.Count()
}

// spawn adds n sprites inside the view, moving, animating and spread over
// three layers.
//
// All three matter. A sprite outside the view is culled before it is drawn, a
// still one never re-picks its source rectangle, and one flat layer lets the
// draw-order sort finish early — so a lazier spawner would measure a scene
// nobody plays.
func spawn(e *wisp.Engine, n int) {
	for range n {
		i := e.Add(wisp.Sprite{
			Alpha:  0.6 + rand.Float64()*0.4,
			Height: tileSize, Image: imageSprites,
			Row:   rowMoveRight,
			State: wisp.StateAnimated | wisp.StateAnimatedLoop | wisp.StateVisible,
			Width: tileSize,
			X:     e.CamX + rand.Float64()*e.Width,
			Y:     e.CamY + rand.Float64()*e.Height,
			Z:     rand.IntN(3),
		})
		e.FrameOffset[i] = rand.IntN(e.Animation.FrameCount)
		vx = append(vx, (rand.Float64()*2-1)*0.15)
		vy = append(vy, (rand.Float64()*2-1)*0.15)
	}
}

// despawn removes the last n spawned sprites. It deletes from the end, so no
// index below it ever shifts.
func despawn(e *wisp.Engine, n int) {
	for range n {
		if e.Count() <= first {
			return
		}
		e.Delete(e.Count() - 1)
		vx = vx[:len(vx)-1]
		vy = vy[:len(vy)-1]
	}
}

// update moves the spawned sprites and reads the lab's own keys.
func update(e *wisp.Engine, dt float64) {
	batch := spawnBatch
	if e.Input.Shift {
		batch *= 10
	}
	switch {
	case e.Input.JustPressed("]"):
		spawn(e, batch)
	case e.Input.JustPressed("["):
		despawn(e, batch)
	case e.Input.JustPressed("0"):
		despawn(e, e.Count()-first)
	case e.Input.JustPressed("p"):
		e.Paused = !e.Paused
	case e.Input.JustPressed("n"):
		build(e)
	case e.Input.JustPressed("b"):
		startRamp(e)
	case e.Input.JustPressed("F3"), e.Input.JustPressed("h"):
		e.Debug.ShowMetrics = !e.Debug.ShowMetrics
	case e.Input.JustPressed("1"):
		e.Impact(e.Feel.Light)
	case e.Input.JustPressed("2"):
		e.Impact(e.Feel.Medium)
	case e.Input.JustPressed("3"):
		e.Impact(e.Feel.Heavy)
	}

	ramp(e, dt)

	// Bounce every spawned sprite off the world edges.
	for n := range vx {
		i := first + n
		e.X[i] += vx[n] * dt
		e.Y[i] += vy[n] * dt
		if e.X[i] < 0 || e.X[i] > worldW {
			vx[n] = -vx[n]
		}
		if e.Y[i] < 0 || e.Y[i] > worldH {
			vy[n] = -vy[n]
		}
	}
}

// startRamp begins, or abandons, the automatic search for the ceiling.
func startRamp(e *wisp.Engine) {
	if ramping {
		ramping = false
		return
	}
	despawn(e, e.Count()-first)
	e.Debug.ShowMetrics = true
	ramping, rampAt, rampCeiling, settled = true, 0, 0, 0
}

// ramp adds sprites until the near-worst frame in the window crosses the
// budget, then stops and keeps the count.
//
// It watches the 99th percentile rather than the average, because a player
// notices the frames an average hides. It waits a second first: the frames
// right after a reset are always slow, and ending on those would report a
// ceiling of zero.
func ramp(e *wisp.Engine, dt float64) {
	if !ramping {
		return
	}
	s := e.Stats()
	settled += dt
	if settled < rampSettle || s.Budget <= 0 {
		return
	}
	if s.P99Ms > s.Budget {
		ramping = false
		rampCeiling = e.Count() - first
		return
	}
	rampAt += dt
	for rampAt >= 1000.0/rampPerSecond {
		rampAt -= 1000.0 / rampPerSecond
		spawn(e, 1)
	}
}

// renderUI draws the lab's own two lines. The tuning menu and the metrics
// overlay are the engine's, and they draw over this.
func renderUI(e *wisp.Engine) {
	const font = "12px system-ui, sans-serif"
	const dim = "rgba(255, 255, 255, 0.55)"

	if !e.Input.Started {
		e.Text(e.Width/2, e.Height/2, "Click to start", "white", "24px system-ui, sans-serif", "center")
		return
	}
	status := "sprites " + itoa(e.Count()-first)
	switch {
	case ramping:
		status += "   ramping..."
	case rampCeiling > 0:
		status += "   ceiling " + itoa(rampCeiling) + " at 60 fps"
	}
	e.Text(8, e.Height-24, status, "white", font, "left")
	e.Text(8, e.Height-10,
		"] [ spawn   0 clear   B ramp   1 2 3 impact   M tune   H metrics   P pause   N reset   F full",
		dim, font, "left")
}

// itoa is here so the lab never reaches for fmt, which costs a TinyGo build
// far more than it looks: strconv alone is a few hundred bytes, fmt is tens of
// kilobytes of reflection.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
