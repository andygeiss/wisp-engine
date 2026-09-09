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
//
// A row is also a tag's index, which is why these still name the right
// animation now that the lab attaches a Sheet: the grid and the Aseprite
// export describe the same twelve animations in the same order, and
// TestGridSheetMatchesTheExport is what says so.
const (
	rowIdleRight = iota
	rowIdleLeft
	rowMoveRight
	rowMoveLeft
)

// The sheet's shape, and the names the Aseprite export gives its first four
// tags. The lab builds its Sheet with GridSheet rather than parsing
// web/static/img/lab.json, because fetching and carrying 27 KB of JSON would
// cost a third of the module's remaining size budget to describe a sheet whose
// every frame is the same size. A game with a packed or per-frame-timed sheet
// parses the export instead; the engine cannot tell the two apart.
const (
	sheetCols = 8
	sheetRows = 12
	frameMS   = 100

	tagRunRight = "run-right"
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
	// frame times, so the first slow frames after a reset do not end it. It
	// has to cover the engine's whole two-second statistics window: at half
	// of it the window still held the reset, whose frames are the ones the
	// 99th percentile reports, and the ramp ended before it had spawned
	// anything.
	rampSettle = 2000.0
)

// A spawned sprite carries its own velocity: the engine moves entities by
// state bits, which is a game's job to set, and bouncing is this lab's game.
//
// spawned holds their entity indices rather than counting from a base index.
// A delete leaves its slot for the next spawn instead of moving everybody
// below it down, so which slot a sprite lands in is the engine's business —
// and the index it hands back is a name that keeps working.
var (
	player  int
	spawned []int
	vx      []float64
	vy      []float64

	// The automatic ramp: it adds sprites until the frame time breaks, then
	// stops and holds the count. That number is what the lab exists to find.
	rampAt float64
	// rampCeiling is -1 until a ramp has finished, so that a ceiling of zero
	// is a result the lab shows rather than one it hides.
	rampCeiling = -1
	// rampDrawn is how many of those the camera actually painted. The world is
	// four times the canvas, so most of the spawned sprites are off-screen and
	// culled: the ceiling on its own overstates what the renderer carried.
	rampDrawn int
	ramping   bool
	settled   float64
)

func main() {
	e := wisp.New(wisp.Config{})
	e.LoadImages("/static/img/lab.png", "/static/img/tiles.png")

	// One sheet, for the sprites. The tileset gets no entry and so keeps the
	// grid convention, which is what a tilemap wants: it indexes its tiles
	// with its own rows and columns, and none of them is an animation.
	sheet := wisp.GridSheet(sheetCols, sheetRows, tileSize, tileSize, frameMS)
	sheet.Tags[rowIdleRight].Name = "idle-right"
	sheet.Tags[rowIdleLeft].Name = "idle-left"
	sheet.Tags[rowMoveRight].Name = tagRunRight
	sheet.Tags[rowMoveLeft].Name = "run-left"
	e.Sheets = []wisp.Sheet{sheet}

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
	spawned, vx, vy = spawned[:0], vx[:0], vy[:0]
	ramping, rampAt, rampCeiling, rampDrawn, settled = false, 0, -1, 0, 0

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
			State: wisp.StateAnimated | wisp.StateAnimatedLoop | wisp.StateVisible,
			Width: tileSize,
			X:     e.CamX + rand.Float64()*e.Width,
			Y:     e.CamY + rand.Float64()*e.Height,
			Z:     rand.IntN(3),
		})
		// A spawned sprite carries no move bits and no bit in RowMask, so
		// nothing in the update will pick a row for it. Play is what names the
		// animation instead of a bare index.
		e.Play(i, tagRunRight)
		e.FrameOffset[i] = rand.IntN(sheetCols)
		spawned = append(spawned, i)
		vx = append(vx, (rand.Float64()*2-1)*0.15)
		vy = append(vy, (rand.Float64()*2-1)*0.15)
	}
}

// despawn removes the last n spawned sprites, newest first, and forgets their
// velocities with them.
func despawn(e *wisp.Engine, n int) {
	for range n {
		last := len(spawned) - 1
		if last < 0 {
			return
		}
		e.Delete(spawned[last])
		spawned = spawned[:last]
		vx = vx[:last]
		vy = vy[:last]
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
		despawn(e, len(spawned))
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
	for n, i := range spawned {
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
	despawn(e, len(spawned))
	e.Debug.ShowMetrics = true
	ramping, rampAt, rampCeiling, rampDrawn, settled = true, 0, -1, 0, 0
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
	if settled < rampSettle || s.Late <= 0 {
		return
	}
	// Against Late, not Budget: the budget is the middle of the window, so
	// half a healthy scene's frames are above it and the ramp would stop on
	// the first check every time.
	if s.P99Ms > s.Late {
		ramping = false
		rampCeiling = len(spawned)
		rampDrawn = s.Drawn
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
	status := "sprites " + itoa(len(spawned))
	switch {
	case ramping:
		status += "   ramping..."
	case rampCeiling >= 0:
		// Both numbers, because they are far apart and the second is the one
		// the renderer actually paid for.
		status += "   ceiling " + itoa(rampCeiling) + " spawned, " +
			itoa(rampDrawn) + " drawn at 60 fps"
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
