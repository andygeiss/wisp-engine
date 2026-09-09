//go:build !js || !wasm

package wisp

import "time"

// backend is the headless twin of the browser half. It exists so go vet, go
// test and staticcheck see the whole API on a developer's machine instead of
// half of it, and so a test can drive the engine without a browser.
//
// It records what it was asked to do, which is what the tests assert against.
// The browser build's backend, in runtime_js.go, is the real one.
type backend struct {
	// Drawn counts the entities the last frame drew.
	drawn int
	// Played records every sound the engine asked for, newest last.
	played []played
	// Rects and texts record the overlay drawing.
	rects int
	texts []string
	// Smoothing is the canvas flag the last frame set.
	smoothing bool

	assets int
	store  map[string]string
	steps  int
}

// played is one call to PlaySound, kept so a test can assert that three hits
// in one frame really are three sounds.
type played struct {
	index  int
	rate   float64
	volume float64
}

func (r *backend) begin(e *Engine) {
	r.smoothing = e.Render.Smoothing
	r.drawn = 0
}

func (r *backend) drawEntities(e *Engine) {
	for _, i := range e.drawOrder {
		if e.State[i]&StateVisible != 0 {
			r.drawn++
		}
	}
}

func (r *backend) end() {}

// now is a monotonic millisecond clock. Off a browser it is the process clock,
// which is enough for the phase timers to add up in a test.
func (r *backend) now() float64 {
	return float64(time.Since(started).Nanoseconds()) / 1e6
}

// started is read once, so now() counts from the first import rather than from
// the epoch and stays inside a float64's exact range for a very long time.
var started = time.Now()

func (r *backend) loadImages(_ *Engine, paths []string) { r.assets += len(paths) }
func (r *backend) loadSounds(_ *Engine, paths []string) { r.assets += len(paths) }

// loading is always false here: there is no network, so every asset has
// arrived by the time the call returns.
func (r *backend) loading() bool { return false }

func (r *backend) pauseMusic() {}
func (r *backend) playMusic(index int, volume float64) {
	r.played = append(r.played, played{index: index, rate: 1, volume: volume})
}

func (r *backend) playSound(index int, volume, rate float64) {
	r.played = append(r.played, played{index: index, rate: rate, volume: volume})
}

func (r *backend) rect(_, _, _, _ float64, _ string) { r.rects++ }

// run steps the engine at a fixed 60 frames a second and never sleeps. It is
// here so Run compiles and a test can drive a few frames, not to run a game.
func (r *backend) run(e *Engine) {
	const frame = 1000.0 / 60
	for !e.stopped {
		e.Step(frame)
		e.draw()
		r.steps++
	}
}

func (r *backend) stop() {}

func (r *backend) storeGet(key string) string { return r.store[key] }

func (r *backend) storeSet(key, value string) {
	if r.store == nil {
		r.store = make(map[string]string)
	}
	r.store[key] = value
}

func (r *backend) stopMusic() {}

func (r *backend) text(_, _ float64, s, _, _, _ string) { r.texts = append(r.texts, s) }

// copyText has nowhere to copy to off a browser, so it keeps the text where a
// test can read it.
func (r *backend) copyText(s string) { r.storeSet("clipboard", s) }

// hostStats fills in the numbers only a browser knows. There is no browser
// here, so it says so rather than reporting a zero somebody might believe.
func (r *backend) hostStats(s *Stats) {
	s.GPU = "n/a — no browser"
	s.LongFrames = -1
}
