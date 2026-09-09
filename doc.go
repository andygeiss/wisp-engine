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
// e.X[i] instead of calling a getter. [Engine.Add] appends to all of them and
// [Engine.Delete] removes from all of them; nothing allocates once the slices
// have grown, which is what keeps the wasm module small and the frame steady.
//
// The engine owns state bits 0 to 13 ([StateVisible] and friends) and a game
// adds its own above bit 15. [Engine.RowForState] maps a state to the sprite
// sheet row that draws it, so the engine never has to know what an "attack" is.
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
