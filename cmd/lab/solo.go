//go:build js && wasm

// The solo scene: today's lab, on this machine alone. It is what ?solo in the
// URL runs, and it is the scene the ramp measures, so nothing about how it
// spawns, bounces or counts changed when the network arrived.

package main

import (
	"math/rand/v2"

	"github.com/andygeiss/wisp-engine"
	"github.com/andygeiss/wisp-engine/internal/lab"
)

// runSolo plays the scene on this engine: the bouncers move from Simulate,
// the keys are read from the frame update, and the ramp does the counting.
func runSolo(e *wisp.Engine) {
	e.RenderUI = func() { renderUI(e) }
	e.Simulate = simulate
	build(e)
	e.Run(func(dt float64) { update(e, dt) })
}

const (
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

// The scene: a player, and the bouncers the lab spawns to measure a frame.
// The bouncers are internal/lab's, so the sprites this scene measures move by
// the same code the server moves them by — from Simulate, on the tick, where
// the draw can blend between two of them.
var (
	player   int
	bouncers *lab.Bouncers

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

// build lays out the scene: a floor, a player, and nothing else. Everything
// after that is spawned by hand, so the count on screen is the count you asked
// for.
func build(e *wisp.Engine) {
	e.Reset()
	bouncers = lab.NewBouncers(e, 0)
	ramping, rampAt, rampCeiling, rampDrawn, settled = false, 0, -1, 0, 0

	// The floor goes below every actor. Sharing a layer with them would not
	// hide it behind them: inside one layer the sort is by baseline, so each
	// tile below a sprite's middle draws after it and repaints its lower half.
	lab.BuildFloor(e)
	player = lab.NewPlayer(e).Entity
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
		bouncers.Add(e.CamX+rand.Float64()*e.Width, e.CamY+rand.Float64()*e.Height)
	}
}

// despawn removes the last n spawned sprites, newest first.
func despawn(n int) {
	for range n {
		if bouncers.RemoveLast() < 0 {
			return
		}
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
		despawn(batch)
	case e.Input.JustPressed("0"):
		despawn(bouncers.Len())
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
}

// simulate bounces every spawned sprite off the world edges. It is the lab's
// whole world, so it belongs on the tick: a sprite moved once per frame is
// already exactly where that frame wants it, and blending it toward a tick it
// never took would drag it backwards — which is why the showcase judders if
// this loop sits in the frame update instead.
func simulate(dt float64) { bouncers.Simulate(dt) }

// startRamp begins, or abandons, the automatic search for the ceiling.
func startRamp(e *wisp.Engine) {
	if ramping {
		ramping = false
		return
	}
	despawn(bouncers.Len())
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
		rampCeiling = bouncers.Len()
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
	if !e.Input.Started {
		e.Text(e.Width/2, e.Height/2, "Click to start", "white", "24px system-ui, sans-serif", "center")
		return
	}
	status := "sprites " + itoa(bouncers.Len())
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
		"] [ spawn   0 clear   B ramp   1 2 3 impact   M tune   H metrics   P pause   N reset   F full   (solo)",
		dim, font, "left")
}
