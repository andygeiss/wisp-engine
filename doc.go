// Package wisp is a 2D pixel-art engine for games that TinyGo compiles to
// WebAssembly.
//
// It is small on purpose: entities in a structure of arrays, a Canvas2D
// renderer, keyboard and mouse, a camera, and no dependencies at all. If you
// can read Go you can read the whole engine in an afternoon.
//
// [New] is the entry point. [Engine.Run] takes the frame loop from there:
//
//	e := wisp.New(wisp.Config{})
//	e.LoadImages("/static/img/hero.png")
//	hero := e.Add(wisp.Sprite{
//		Height: 32, State: wisp.StateVisible, Width: 32, X: 320, Y: 180,
//	})
//	e.CamTarget, e.InputTarget = hero, hero
//	e.Run(func(dt float64) {
//		if e.Input.JustPressed(" ") {
//			e.Impact(e.Feel.Heavy)
//		}
//	})
//
// # Tuning
//
// Every number that decides how a game feels lives in one [Settings] value,
// embedded in the engine. Press M while the game runs and a menu appears on
// the canvas that edits it, remembers it across reloads, and copies it back
// out as Go source you paste into your game. That is what the engine is for:
// a feel is found by trying it, and a rebuild between tries is too slow.
//
// A hit is three named strengths rather than numbers at the call site:
//
//	e.Impact(e.Feel.Light)  // a connect
//	e.Impact(e.Feel.Heavy)  // a finisher
//
// # Entities
//
// An entity is an index into every slice at once, so a game reaches into
// e.X[i] instead of calling a getter. [Engine.Add] fills a slot and
// [Engine.Delete] empties one and keeps it for the next entity; nothing
// allocates once the slices have grown, which is what keeps the wasm module
// small and the frame steady.
//
// Keeping the slot is what makes an index worth holding: a delete moves
// nothing, so an index still means the entity it meant last frame. Iterate to
// [Engine.Slots] and skip the holes with [Engine.Live]; [Engine.Count] is how
// many entities there are, which is a smaller number. A slot is reused
// eventually, so an index that has to outlive a delete — one in a save, or in
// a packet — travels as an [ID] and is resolved back with [Engine.Index].
//
// The engine owns state bits 0 to 13 ([StateVisible] and friends) and a game
// adds its own above bit 15. [Engine.RowForState] maps a state to the sprite
// sheet row that draws it, so the engine never has to know what an "attack" is.
//
// # Time
//
// A frame and a tick are not the same thing. [Engine.Run] calls the update it
// was given once per frame, on however long the frame took: read the keys
// there, spawn things there, draw a HUD there. The world moves in
// [Engine.Simulate], which runs at [TimeSettings.TickRate] a second with the
// same dt every time, so how far anything goes stops depending on how fast the
// machine drawing it is:
//
//	e.Simulate = func(dt float64) {
//		if e.Input.Down("z") {
//			e.X[hero] += 0.2 * dt // the same distance on any display
//		}
//	}
//
// A frame pays for whole ticks and carries the remainder, so a frame may run
// two ticks or none. Everything the player only looks at — the camera, the
// shake, the animations — stays on the frame, because none of it has to agree
// with anybody.
//
// # Platforms
//
// The browser half is behind a js and wasm build tag. Everything else builds
// and tests anywhere, against a headless stand-in, so go test and go vet see
// the whole API on a developer's machine.
//
// Nothing here is safe for concurrent use. The browser calls the frame loop on
// one goroutine, which is the only way it is meant to be driven.
package wisp
