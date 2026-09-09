package wisp

import "slices"

// Input is the keyboard and mouse as of this frame.
//
// A key is named the way the browser names it: one lower-case character for a
// letter or digit ("w", "3"), and the DOM name for everything else
// ("ArrowLeft", "Enter", "Escape", "Tab"). Space is " ".
//
// [Input.Down] answers "is it held", [Input.JustPressed] answers "did it go
// down this frame". The second one is why a game no longer has to clear a key
// by hand after acting on it.
type Input struct {
	// Alt, Ctrl, Meta and Shift are the modifier keys, as of the last event.
	Alt   bool
	Ctrl  bool
	Meta  bool
	Shift bool

	// MouseDown is true while a mouse button is held on the canvas.
	MouseDown bool
	// MouseX and MouseY are the pointer in world coordinates.
	MouseX float64
	MouseY float64

	// Started turns true on the first key or click and stays true. Browsers
	// refuse to play sound before it, so a game waits for it.
	Started bool

	down     map[string]bool
	locked   bool
	pressed  map[string]bool
	released map[string]bool
}

// Down reports whether key is held right now.
//
// It answers false for every key while the tuning menu is open. The menu takes
// the keyboard so a game cannot be steered underneath it; the game's own logic
// keeps running, which is what lets a camera be judged while it is tuned.
func (in *Input) Down(key string) bool { return !in.locked && in.down[key] }

// JustPressed reports whether key went down during this frame. It is true for
// exactly one frame, so a game never has to clear a key by hand.
func (in *Input) JustPressed(key string) bool { return !in.locked && in.pressed[key] }

// JustReleased reports whether key came up during this frame.
func (in *Input) JustReleased(key string) bool { return !in.locked && in.released[key] }

// AnyDown reports whether any of the keys is held.
func (in *Input) AnyDown(keys ...string) bool {
	return slices.ContainsFunc(keys, in.Down)
}

// pressedRaw is JustPressed without the lock. The menu reads its own keys
// through it, because the lock is what the menu put there.
func (in *Input) pressedRaw(key string) bool { return in.pressed[key] }

// Key records a press or a release. The browser half calls it; a test calls it
// to drive the engine without a browser.
func (in *Input) Key(key string, isDown bool) {
	key = normalizeKey(key)
	if key == "" {
		return
	}
	if in.down == nil {
		in.down = make(map[string]bool)
		in.pressed = make(map[string]bool)
		in.released = make(map[string]bool)
	}
	if isDown {
		in.Started = true
		if !in.down[key] {
			in.pressed[key] = true
		}
		in.down[key] = true
		return
	}
	if in.down[key] {
		in.released[key] = true
	}
	delete(in.down, key)
}

// endFrame forgets this frame's edges. The engine calls it after every tick,
// which is what makes JustPressed mean "this frame" and not "since the last
// time somebody looked".
func (in *Input) endFrame() {
	clear(in.pressed)
	clear(in.released)
}

// moveAxis returns the direction the movement keys are asking for, each
// component -1, 0 or 1. WASD and the arrow keys both work, so a player who
// reaches for either is right.
func (in *Input) moveAxis() (dx, dy float64) {
	if in.AnyDown("a", "ArrowLeft") {
		dx--
	}
	if in.AnyDown("d", "ArrowRight") {
		dx++
	}
	if in.AnyDown("w", "ArrowUp") {
		dy--
	}
	if in.AnyDown("s", "ArrowDown") {
		dy++
	}
	return dx, dy
}

// normalizeKey folds a printable key to lower case and leaves the named keys
// alone, so "W" and "w" are one key and "ArrowLeft" keeps its name.
func normalizeKey(key string) string {
	if len(key) != 1 {
		return key
	}
	if c := key[0]; c >= 'A' && c <= 'Z' {
		return string(c - 'A' + 'a')
	}
	return key
}

// SetModifiers records the modifier keys an event arrived with. The browser
// half calls it; a test calls it to drive the menu's coarse and fine steps.
func (in *Input) SetModifiers(alt, ctrl, meta, shift bool) {
	in.Alt, in.Ctrl, in.Meta, in.Shift = alt, ctrl, meta, shift
}
