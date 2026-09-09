# Extract the Wisp engine into a standalone, tunable module

**Status: milestones 1 to 4 are built and green. 5, 6 and 7 are not started.**
This file is both the plan and the record — what was decided, what was built,
what changed while building it, and what is left.

**Two changes landed ahead of milestone 7**, the networked game: an entity index
is now a name the engine keeps, and the simulation runs on a fixed tick instead
of on however long the frame took. Both stand on their own merit and both are
green — *Two changes networking needs* has them. Milestone 7 itself is a brief
and nothing else, and it needs a decision about this project's job before it is
more than that.

**The lab has since been built and run.** TinyGo and `wasm-opt` are installed,
`make wasm` produces a 228 KB module, every gate is green including the two
that need the network, and the scene has rendered in a browser. That first run
is what turned up the floor-layer bug in *Fixes on the first real run*.

## Context

`game-jam-template` already contained an engine, and its own pause screen
already called it "Wisp Engine". It was 802 lines in `internal/engine/`, split
cleanly in two:

- `entity.go` (474 lines, no build tag) — entity storage as a structure of
  arrays, camera, keyboard state, the per-frame state machine.
- `runtime.go` (328 lines, `//go:build js && wasm`) — Canvas2D, `Image` and
  `Audio` loading, input events, the `requestAnimationFrame` loop.

Zero third-party dependencies, TinyGo 0.42 to a 309 KB module, and stuck inside
one game.

**The problem was the feedback loop.** Every number that decided how the game
*felt* was a `const` or a literal at a call site: hit stop 70 ms, shake
magnitude 4.0 for 150 ms, 8 animation frames of 100 ms, a 12 px hit-box margin,
0.125 px/ms movement. Changing one meant edit, `tinygo build`, `wasm-opt`,
reload. The template's own README admitted it: *"Rebalance — every cooldown,
life count and speed lives in the constants at the top of
`cmd/client/main.go`."*

Two more things blocked reuse. The engine was entirely package-level mutable
globals, so it could not be published and its tests could not run in parallel.
And the sprite format was a rigid convention — 8 frames of 100 ms, one row per
animation — so Aseprite's real output could not be used.

**What this delivers:** `github.com/andygeiss/wisp-engine` — a zero-dependency
2D pixel-art engine for Go compiled to WebAssembly, with every feel knob in one
`Settings` value that a canvas-drawn menu edits live, a metrics overlay that
answers *"how many sprites before it drops below 60?"*, and a playground you
open and press keys in.

## The brief

- **Job:** A solo developer tunes a 2D pixel-art browser game's feel in the
  browser, sees what it costs, and pastes the tuned numbers back into their
  source.
- **Why:** Every feel change was a TinyGo rebuild, and no number anywhere said
  what a change cost. Both are why the game in the template still feels like its
  first draft.
- **Guardrails:** Zero third-party dependencies. Everything compiles under
  TinyGo 0.42 — 309 KB is the number to protect; the module measures 229 KB. All overlay UI is drawn on the
  canvas, never as DOM, so it survives fullscreen. `game-jam-template` is not
  touched until the last milestone.
- **Done means:** `make check` and `make ci` are green; the library checklist is
  walked with every box checked or waived in the README; the lab runs in a
  browser and `M`, the spawn key and the overlay all do what they say;
  `game-jam-template` imports the module and still plays.

## Where it stands

| Milestone | State |
|---|---|
| 1. The module, the `*Engine` struct, the migrated tests | **done** |
| 2. Something to open: `cmd/lab`, `cmd/serve`, the host page | **done** |
| 3. `Settings` and the `M` menu | **done** |
| 4. The metrics overlay and the sprite spawner | **done** |
| 5. Aseprite sheets | **not started** — the design is below |
| 6. `game-jam-template` imports the module | **not started** |
| 7. A networked game | **not started** — a brief, below, and two of its groundwork changes already landed |

Two follow-ups, each gated on evidence rather than scheduled:

- **A WebGL2 batched renderer**, if and when the number `B` produces is too low.
  It is not: `B` reports about 34,000 sprites at 60 fps over two runs, 9,045 of
  them actually drawn. The overlay's *sort / draw* row splits the frame, so a
  rerun says how much of it a new renderer could even reach.
- **A native backend** for true OS-level CPU, RAM and GPU numbers. A whole
  second platform layer, and a large one.

### Gates

Green, all of them: `gofmt`, `go vet`, `GOOS=js GOARCH=wasm go vet`,
`go fix -diff`, `staticcheck`, `govulncheck`, `go mod tidy -diff`,
`go test -race -shuffle=on`, `CGO_ENABLED=0 go build`. `make ci` runs that same
list against the commit and is green too. 49 test functions: 44 tests, four
examples, and one fuzz target with its seeds.

`make wasm` has run. `web/static/lab.wasm` is committed at 234,882 bytes — 229
KB, under the 320,000-byte gate — built with TinyGo 0.42.0 and LLVM 22.1.4. The
free list and the fixed tick cost 586 bytes of that between them.

**`staticcheck` needed a fix before it would pass, and it was not noise.**
`fullscreenAt` and `claimsKey` are reached only from `runtime_js.go`, so the
host build saw them as unused (U1000) on correct code. The `go vet` trick does
not extend to it: `go run tool@latest` under `GOOS=js` builds the tool for that
target as well, and a js/wasm `staticcheck` binary cannot execute, so there is
no second line to add. Both are exercised on the host instead — which is what
`host.go` exists for — and the fullscreen debounce moved out of
`toggleFullscreen` into `Engine.allowFullscreen`, so the rule can be tested
where there is no fullscreen to ask for.

## Decisions taken

| Question | Answer |
|---|---|
| Engine state | An `*Engine` struct with `Settings` embedded. `e.X[i]` keeps the direct slice access; no getters. |
| Repo shape | One repo: the engine at the module root, plus `cmd/lab`, `cmd/serve` and `web/`. The library rule against a `main` package is waived on the record; nothing is embedded, so that half needs no waiver. |
| Renderer | Canvas2D now. The overlay produces the evidence for a WebGL2 batcher later. |
| Metrics | Honest browser proxies only. CPU% and GPU% read `n/a — not exposed by browsers`, never a made-up number. |
| Sheet parsing | A hand-written scanner, not `encoding/json`. See *Aseprite sheets*. |

## What the repository holds

```
wisp-engine/
├── assets/                  the .aseprite sources; `make sheets` exports them
├── camera.go            140 follow, dead zone, look-ahead, bounds, shake
├── cmd/
│   ├── lab/main.go      313 the playground (js && wasm)
│   └── serve/main.go    230 the static file server behind `make run`
├── DESIGN.md                the page's tokens
├── doc.go                82 the package doc
├── engine.go            389 Engine, Config, New, Step, Tick, Impact, HitStop
├── entity.go            314 Sprite, Tilemap, ID, Add, Delete, Slots, Live
├── example_test.go       75 runnable Examples for pkg.go.dev
├── go.mod                   go 1.27, no requires
├── host.go              115 the same API, headless (!js || !wasm)
├── input.go             129 key state, edge detection, the menu's lock
├── internal_test.go     536 draw order, layering, menu widths, the frame ring
├── knobs.go             322 the knob table, GoLiteral, the text format
├── LICENSE                  MIT
├── Makefile                 baseline Makefile + sheets and wasm targets
├── menu.go              358 the M overlay, canvas-drawn
├── metrics.go           342 the frame ring, percentiles, the H overlay
├── PLAN.md                  this file
├── random.go              8 the engine's one source of randomness
├── README.md                install, the 30-second example, waived rules
├── render.go            119 Run, Stop, Rect, Text, sound, the frame's paint
├── runtime_js.go        464 canvas, events, audio, rAF loop (js && wasm)
├── settings.go          211 Settings and its eight groups, Defaults
├── settings_test.go     244 the literal, the coverage net, the fuzz target
├── SPEC.md                  job, why, guardrails, done means
├── state.go             191 the state bits, movement, animation, draw order
├── web/
│   ├── static/              app.css, wasm_app.js, the art, lab.wasm
│   └── templates/index.html the host page
└── wisp_test.go        1091 the consumer's view: entities, camera, input, menu
```

2,720 lines of engine that build anywhere, 464 behind the browser tag, 1,946 of
tests, 543 of lab and server.

**`cmd/serve` reads `web/` from disk; nothing is embedded.** Editing the
stylesheet and reloading is then the whole loop instead of a rebuild. And the
root package is the library — an `assets.go` there would make every consumer of
`wisp` embed the lab's page and its wasm binary. The server opens the tree with
`os.Root`, so a symlink under `web/` cannot serve anything outside it.

`host.go` is the piece the template lacked: it gives the browser half a headless
twin, so `go vet`, `go test` and `make check` see the whole API on the host
instead of half of it. It also records what it was asked to draw and play, which
is what several tests assert against.

**The render seam.** Everything that draws lives in `runtime_js.go`, and every
`syscall/js` call in the module is in that one file. Swapping Canvas2D for
WebGL2 later rewrites it and nothing else. There is no `renderer` interface
until a second renderer exists — an interface with one implementation is the
abstraction layer `stack/go.md` bans.

## The engine as a struct

`Settings` is **embedded** in `Engine`, which buys both shapes at once:

```go
e := wisp.New(wisp.Config{})
e.Feel.Heavy.ShakeMagnitude = 12 // promoted: short, what you write while tuning
e.Settings = wisp.Defaults()     // the whole set: what the menu resets and saves
```

The 14 slices are exported fields with the `Entity` prefix dropped — `e.X[i]`,
`e.State[i]`, `e.Alpha[i]` — so the "reach into the array instead of calling a
getter" ergonomics the old doc comment defended survives intact. Every test gets
its own engine and runs with `t.Parallel()`.

Three shapes changed while building, against the original sketch:

- **`Tick(elapsed, update)` became `Run(update)` plus `Step(elapsed)`.** `Run`
  stores the callback and blocks until `Stop`, so a game's `main` is one call and
  the `select {}` that used to keep the Go runtime alive is the engine's problem
  now. `Step` is the whole frame, and a test drives it directly.
- **`Sprite` and `Tilemap` structs replaced the positional arguments.**
  `AddEntity(state, imgIndex, imgCol, imgRow, w, h, x, y, alpha, z)` had two
  adjacent `int`s that were trivially swappable. `Alpha` and `SpeedFactor` of 0
  mean 1, because an entity added invisible or motionless is almost always a
  mistake.
- **The platform type is `backend`, not `runtime`.** The obvious name collides
  with the standard library's `runtime`, which the metrics need for
  `ReadMemStats`.

## Fixes that landed during the move

Nine, each with the test that pins it.

| Fix | What happened |
|---|---|
| `BoundingBox` shrank left and top only — `r = l + w - margin` where `l` already added it, so `r = x + w/2`. A 32 px sprite got a 20x20 box offset 6 px down and right. | Symmetric formula, and the default margin halved from 12 to 6 so the box stays 20x20 and the game's balance is unchanged. `TestBoundingBox` is now a table over four margins and asserts the box's centre is the sprite's own. |
| No `ctx.imageSmoothingEnabled = false`. Sharpness rested entirely on the CSS. | `Render.Smoothing`, default off, **set every frame** — setting a canvas's width or height resets the whole 2D context, so a once-at-startup version silently stops working after the first fullscreen toggle. |
| Shake kept rattling at full amplitude during a hit stop, because `updateCamera(0)` still re-randomised. Emergent, not designed. | `updateCamera(dt, elapsed)` takes both clocks. The world is simulation and runs on scaled time; the shake is presentation and runs on wall time. `Shake.IgnoresHitStop` makes it a decision, and two tests cover both answers. |
| Animation advanced inside the draw pass, behind the viewport cull, so off-screen entities froze. | Moved into `Step`, with `Animation.CullOffscreen` (default off) for the old behaviour. The test that proves it could not have been written before. |
| `PlaySound` silently dropped a re-trigger while the sound still played. Three monsters dying in one swing made one sound. | A voice pool of `Audio.Voices` elements per sound, taken round-robin, plus `Audio.PitchJitter` on the playback rate. `PlaySound` and `PlayMusic` split, because they were two jobs sharing one `paused` guard. |
| No `onerror` on `Image` or `Audio`: one 404 hung on "Loading..." for ever. | Both count as arrived and record the path. `Engine.Failed` returns them and the loading screen names the first one. |
| `camBoundsSet` was initialised `true` and never set false. | Deleted. The clamp always runs, which is what the flag pretended to gate. |
| Games hand-cleared key bools (`engine.KeyN = false`) — the most-copied boilerplate. | `JustPressed` and `JustReleased`, cleared by `Step`. A press and release inside one frame still reports both edges, which a `down`/`prev` pair would lose. |
| `F` was stolen for fullscreen before a game saw it, and `preventDefault` fired on every event including `mousemove`. | `Config.FullscreenKey` and `Config.MenuKey`, either of which takes `KeyNone`. Only a key the engine claims is swallowed; `mousemove` never is. Auto-repeat no longer counts as a new press. |

## Fixes on the first real run

Six, from installing TinyGo and opening the page for the first time. Every
one of them was invisible to `make check`, because a green gate says nothing
about a tree nobody serves, a scene nobody looks at, a line of text nobody
measures, or a number nobody reads.

| Fix | What happened |
|---|---|
| `GET /static/js/wasm_exec.js` answered 404, and `make wasm` had never produced anything. `sheets` and `wasm` wrote under `cmd/serve/web/static/`, a directory that does not exist — so the target died on its first `cp`, silently, every time. | Both targets now write to `web/`, the tree `cmd/serve` actually reads and where the committed art already lived. It is also where the baseline's `go-project-layout.md` puts it. |
| `WASM_MAX_BYTES` was declared and never read. Its own comment called it "the headline number, as a gate", and the `wasm` target's said "the size check is last" — but nothing compared anything. | The target's last line is the comparison. A claim nothing checks is a memory, including this one. |
| **The menu's hint line was cut off at the right border**, showing "T te" for "T test". | The panel is pinned to the canvas's right edge and its text is left-aligned inside it, so a line wider than the panel is not clipped to the panel — it is drawn past the edge of the canvas and its tail is gone. The hint was 32 characters where 29 fit. `menuCols` now states the budget with its arithmetic, the hint is `←→ tune  ⇧ x10  C copy  T hit`, and `TestMenuTextFits` checks every fixed string, every knob's footer and every knob row against it, counting runes rather than bytes — `←` is three bytes and one column. |
| **The frame budget rose with the load, so nothing could ever be reported as slow.** At 20,000 sprites `Budget` read 200 ms on a display that was still 60 Hz. | `budget()` was the middle of the current window, which measures the display only while the scene keeps up: once every frame is slow the middle rises with them, slow becomes the new normal, and the overlay calls a scene running at five frames a second comfortably inside budget. It is now the *lowest* the middle has been — a property of the hardware, free to fall as the true interval is learned but never to rise with the load. The middle rather than the single fastest frame, because one coalesced callback would otherwise latch a two-millisecond "display" for the rest of the run and make every frame after it late. |
| **The ramp never produced a number, and neither did anything else that judged a frame.** `B` reset the scene, waited, stopped instantly and displayed nothing. | `budget()` is the *middle* of the two-second window, so about half a healthy scene's frames sit just above it — and three places compared against it as though it were a ceiling. The ramp stopped the first time it looked, at zero sprites, which `renderUI` then hid because it only showed a ceiling above zero. The `p50 / p99` row was permanently amber and half the sparkline bars with it. `lateFrame = 1.5` now names the rule the dropped-frame counter already used, `Stats.Late` carries it out to the lab, and `rampSettle` covers the whole window instead of half of it — at 1000 ms the window still held the reset, whose frames are exactly the ones the 99th percentile reports. `rampCeiling` starts at -1, so a ceiling of zero is a result the lab shows rather than one it swallows. |
| **The floor drew over the sprites.** About a third of every spawned batch came out with its lower half repainted by the tiles under it, which reads on screen as sprites that are clipped and half transparent. | `AddTilemap` was called without `Z`, so 960 floor tiles landed on layer 0 — and `spawn` picks `Z` from `rand.IntN(3)`, so one sprite in three joined them there. Inside a layer the sort is by baseline, so every tile below a sprite's middle draws after it. The floor moved to `zFloor = -1`. `TestTilemapBelowStaysBelow` pins it, and fails with 8 of 16 tiles on top when the layer is shared. |

The floor is the one worth remembering: the engine was right at every step. The
painter's sort, the alpha cache, the viewport cull and the grid arithmetic all
did exactly what they promise. It was the *scene* that put a floor and its
actors on one layer, and no amount of engine testing would have found it.

The transparency in that scene is not a bug and was never one: `spawn` asks for
`Alpha: 0.6 + rand.Float64()*0.4`, so every spawned sprite is 60 to 100 per
cent opaque on purpose. It is what made the overdraw look like a rendering
fault rather than a layering one.

## Two changes networking needs

Both landed before milestone 7 was decided, because both are worth having
whether or not it happens. Neither is network code; both are things the engine
got wrong for a game that is only ever played by one person at one keyboard.

### An entity index is now a name the engine keeps

`Delete` used to call `slices.Delete` on all fourteen arrays, so every entity
above the hole moved down one. Its own doc comment admitted the cost: *"a game
holding indices must shift them too"*. Nobody can. An index in a variable meant
its neighbour the moment somebody else was deleted, and an index in a packet
could not survive the trip.

`Delete` now empties the slot and keeps it, and `Add` takes it back off a free
list. Nothing moves, so an index still means what it meant last frame.

| Before | Now |
|---|---|
| `Count()` was `len(State)`, and *"every index below it is valid"* | `Count()` is the number of entities. `Slots()` is the range to iterate and `Live(i)` skips the holes |
| `Delete` was O(n) in the arrays plus an O(n) `drawOrder` rebuild | O(1) in the arrays, one `slices.Index` in the draw order |
| Deleting entity 1 renumbered entity 2 to 1 | Entity 2 is still entity 2 |
| `CamTarget` and `InputTarget` were shifted by `shiftIndex` | Cleared to -1 when they point at the entity that went; `shiftIndex` is deleted |
| Deleting the same entity twice deleted its neighbour | The second delete does nothing |

A slot is reused, so a bare index is still not a name that outlives its entity.
`ID` is: one `uint64` carrying the slot and the generation stamped on the entity
that filled it, `IDOf` to get one and `Index` to resolve it back. A stale ID
resolves to -1 rather than to the stranger that took the slot, and one counter
for the whole engine — rather than one per slot — means an ID from before a
`Reset` cannot come back to life either. It wraps after four billion entities,
which is longer than a browser tab lives.

The emptied slot is a 0x0 invisible entity with no state bits, which is not
tidiness: `updateStates` skips anything with no move bits and no bit in
`RowMask`, `advanceAnimations` skips anything not animated, and the draw skips
anything not visible. A dead slot is already invisible to all three, so the
holes cost no new branch in any hot loop.

**The lab stopped deriving indices from a base.** It kept `first int` and
addressed spawned sprite `n` as `first + n`, which happens to still work — the
free list is last-in-first-out and the lab despawns in the reverse of the order
it spawns — but only by accident of both. It now keeps the indices `Add` handed
back in a `spawned []int`, and `first` is gone: every `e.Count() - first` was
`len(spawned)` all along.

### The simulation runs on a fixed tick

`Step` moved the world by however long the frame took. Two machines with
different displays therefore ran different simulations from the same input,
which is the thing that makes prediction and reconciliation impossible — and
the reason has nothing to do with networking: it is also why the same replay
gave two answers on two laptops.

`Step` is still the frame. It now spends the frame's world time on whole
`Tick` calls of `1000/Time.TickRate` ms and carries the remainder, so a frame
runs two ticks, or one, or none, and the average rate is exactly right.

The split is the one the engine already made for the shake — simulation against
presentation:

| Runs per tick, fixed dt | Runs per frame, the frame's dt |
|---|---|
| `Engine.Simulate`, the game's own world | the update given to `Run`: keys, spawning, HUD |
| `updateStates` — movement, facing, the sheet row | `updateCamera` — follow, dead zone, shake |
| | `advanceAnimations` — frame offsets are presentation |
| | the tuning menu, the input edges, the metrics |

`Simulate` is a new field next to `RenderUI`. The update given to `Run` keeps
its documented contract exactly: it is called every frame with the world's own
dt, which is 0 while paused, so a paused game still reads the key that leaves
the pause. That is why input *edges* belong there and not in `Simulate` — a
frame can carry two ticks or none, so a `JustPressed` read inside `Simulate`
would fire twice or not at all.

`Tick(dt)` is exported for the same reason `Step` is. A test drives it
directly; so, one day, does a replay of a tick whose input arrived from
somewhere else.

`Time.TickRate` is the 39th knob, default 60. It is in the menu because every
field of `Settings` is, but its doc comment says what the menu cannot: two
machines simulating one world have to agree on it, so a networked game takes it
from the server rather than from a saved menu.

**What this does not do yet, and it is visible.** There is no interpolation
between ticks. At 60 ticks on a 60 Hz display the frame jitter means the
occasional frame runs two ticks or none, which is a one-tick pop — about two
pixels at the default speed, and pixel-art snapping hides most of it. On a 144
Hz display it is a third of the frames. The fix is a previous position per
entity and a lerp in the draw, which is 16 bytes an entity and a change to the
one hot loop in `runtime_js.go`; it is not done, and the lab's own bounce is
still in the per-frame update rather than in `Simulate` because moving it there
before interpolation exists would put the judder into the showcase.

**The ramp number needs re-reading.** `updateStates` moved from every frame to
every tick, so the per-frame cost changed. It should not have changed much —
the spawned sprites carry no move bits and no `RowMask` bits, so that loop was
already skipping them, and `advanceAnimations`, the pass that does touch all of
them, is still per frame. But 34,476 was measured against the old shape and is
now a number about a build that no longer exists.

## `Settings` — the point of the project

Eight groups, alphabetical, every field a number or a bool so one menu can edit
all of them. **39 knobs across 11 nesting levels**, because `Feel` holds four
sub-structs of its own.

```go
type Settings struct {
	Animation AnimationSettings // CullOffscreen, FrameCount, FrameDuration
	Audio     AudioSettings     // MusicVolume, PitchJitter, SfxVolume, Voices, Volume
	Camera    CameraSettings    // DeadzoneHeight/Width, Lookahead, Smoothing, Snap
	Debug     DebugSettings     // ShowHitBoxes, ShowMetrics
	Feel      FeelSettings      // Heavy, Light, Medium, Shake
	Render    RenderSettings    // Background, PixelSnap, Smoothing
	Time      TimeSettings      // FullscreenDebounce, MaxStep, Scale
	World     WorldSettings     // HitBoxMargin, Speed
}
```

**Feel is three named strengths, not scattered numbers.** The template's hit
stops were `{70, 100, 150}` ms and its shakes `{2.5, 4.0, 10.0}` px, spread over
six call sites. The author's own tuning rule survived only in a deleted git
comment: *"big finisher: 150-200, light hits: 40-70"*. Wisp makes that rule the
API:

```go
type ImpactSettings struct {
	HitStopDuration float64 //   0..500 ms,  step 5
	HitStopScale    float64 //   0..1,       step 0.05 — 0 freezes, 0.1 is slow motion
	ShakeDuration   float64 //   0..2000 ms, step 10
	ShakeMagnitude  float64 //   0..32 px,   step 0.5
}

type FeelSettings struct {
	Heavy  ImpactSettings // 150 ms stop, 10 px for 400 ms
	Light  ImpactSettings //  70 ms stop, 2.5 px for 100 ms
	Medium ImpactSettings // 100 ms stop, 4 px for 150 ms
	Shake  ShakeSettings  // the curve all three share
}
```

so a game writes `e.Impact(e.Feel.Heavy)` at the boss's arrival and nothing else.
No literal at the call site, so nothing to rebuild when it feels wrong.

Three knobs are worth naming because they did not exist before:

- **`Camera.Smoothing`** is a catch-up rate per second, applied as
  `1 - exp(-Smoothing*dt)`, so the feel does not change between a 60 Hz screen
  and a 144 Hz one. 0 is the hard snap the engine used to do.
- **`Shake.Decay`** shapes the fade: 1 is a straight line, 2 drops fast then
  trails off. The old shake held full strength and cut.
- **`Shake.Frequency`** re-picks the offset 30 times a second rather than every
  frame. A new offset every frame is white noise, which reads as static and looks
  different on every display.

### The knob table

One hand-written table drives the menu, the saved format and the Go literal, so a
knob cannot exist in one and not the others.

```go
type knob struct {
	Field          string
	Group          string
	Kind           knobKind // knobFloat, knobInt, knobBool
	Min, Max, Step float64
	Get            func(*Settings) float64
	Set            func(*Settings, float64)
}
```

No reflection — TinyGo pays for that in bytes. **The safety net is a test that
uses reflection freely**, because it runs under ordinary Go on the host: it walks
every leaf field of `Settings` and fails when one has no knob. Add a field,
forget the knob, and the menu would silently never show it. That check costs the
shipped module nothing.

## The tuning menu — `M`

Drawn on the canvas with `Rect` and `Text`, because `DESIGN.md`'s rule is that
overlay UI lives inside the canvas so it survives fullscreen — and the CSP bans
the inline script a DOM panel would want.

**The world keeps running while it is open.** A camera or a hit stop can only be
judged in motion. What the menu does instead is take the keyboard: every
`Input.Down` and `Input.JustPressed` a game asks about answers false while it is
open, so the player cannot steer and the game's own logic is untouched.

| Key | What it does |
|---|---|
| `M` | opens and closes it, `Escape` closes |
| up / down | move, `Enter` opens a group or flips a switch |
| left / right | one step, `Shift` ten, `Ctrl` a tenth |
| `Backspace` | reset this knob, with `Shift` everything |
| `C` | copy the whole settings value to the clipboard, as Go |
| `T` | fire a test impact, so you can feel a change without playing |

`C` is the feature that closes the loop. It writes a complete, gofmt-aligned
`wisp.Settings{...}` literal — every field, so pasting it over an old one leaves
nothing behind. A test parses the output with `go/parser` to prove what lands on
the clipboard is Go somebody can paste. `navigator.clipboard` needs a secure
context, and `http://127.0.0.1` is one, so `make run` is enough.

Changes are written to `localStorage` half a second after the last one, so a held
arrow key does not write on every frame. The format is one `name=value` line per
knob, not JSON: it never fails, an unknown name is skipped, a missing one keeps
its default, and every value is clamped to its own range — so a hand-edited
`Time.Scale=1e308` typed into a console cannot brick a game. That clamping is
what the fuzz target exists to prove.

A game that passes `MenuKey: wisp.KeyNone` gets no menu **and** no storage
access, so a shipped build always starts on its compiled-in settings.

## The metrics overlay — `H`

**Say plainly what a browser will not tell you.** There is no CPU-utilization API
and no GPU-utilization API in any browser; the WebGL timer-query extension that
would give GPU frame time is switched off in Chrome for almost everyone. Those
two rows read `n/a — not exposed by browsers`, and a test asserts they never
arrive as a plausible zero.

| Row | Where it comes from |
|---|---|
| fps, frame p50 / p99 | `performance.now()` deltas into a 120-slot ring; percentiles sort a preallocated scratch array, so the overlay allocates nothing |
| update / draw ms | `now()` at the phase boundaries. The overlays are timed separately and excluded, so the overlay never hides inside the number it reports |
| main thread | work ms over the budget, where the budget is the **measured** median frame interval, not a hardcoded 16.7 — a 120 Hz display has half of that |
| entities / drawn | drawn is after the camera threw the off-screen ones away |
| go heap | `runtime.ReadMemStats`, sampled **once a second**: under TinyGo's conservative collector this walks the heap and would show up in the frame time it is meant to explain |
| wasm mem | the loader publishes `instance.exports.memory`; Go reads `.buffer.byteLength` fresh each time, because growing the memory detaches the old buffer |
| js heap | `performance.memory.usedJSHeapSize` — Chromium only, `n/a` elsewhere |
| gpu | `WEBGL_debug_renderer_info` on a throwaway context that is then released; prints `(masked)` when the browser masks it |
| long frames | `PerformanceObserver` on `long-animation-frame`, falling back to `longtask` — Chromium only, `-1` elsewhere |
| cpu / gpu load | `n/a — not exposed by browsers` |

Below the rows, a 60-bar frame-time graph: amber past the budget, red past twice
it. One fill per bar — merging runs of one colour would be fewer calls into the
browser, but every bar has its own height, and the heights are the reason to draw
a graph.

`TestMetricsAllocFree` asserts `AllocsPerRun` is 0 for adding a frame and taking
a percentile. It is the one test that does not run in parallel, because
`AllocsPerRun` counts the whole process.

**Still unverified, and it needs TinyGo:** which `MemStats` fields TinyGo
actually fills. Every field it does not assign reads 0. Check with
`grep -A 25 'func ReadMemStats' "$(tinygo env TINYGOROOT)"/src/runtime/gc_*.go`
before trusting the heap row, and delete the row rather than ship a constant.

## The lab — `cmd/lab`

One scene that exists to be measured: a 40x24 tiled floor, a player you drive
with WASD, and however many sprites you have asked for.

The spawner produces sprites that actually cost something. Off-screen entities
are culled and, with `CullOffscreen` on, do not even animate — so a naive spawner
would measure nothing. Spawned sprites start inside the view, carry a velocity
that bounces them off the world edges, animate from a random frame, and take a
spread of Z values so the per-frame draw-order sort does real work.

| Key | What it does |
|---|---|
| `]` `[` | add / remove 100 sprites, with `Shift` 1000 |
| `0` | clear them all |
| `B` | **ramp**: add 100 a second until p99 crosses the budget, then stop and hold the count |
| `1` `2` `3` | a light, medium and heavy impact |
| `M` | the tuning menu, `H` or `F3` the metrics overlay |
| `P` | pause, `N` reset, `F` fullscreen, `WASD` move |

`B` is the answer to "how many sprites before it drops below 60?" — it finds the
number instead of making you hunt for it. It watches the 99th percentile rather
than the average, because a player notices the frames an average hides, and it
waits a second first: the frames right after a reset are always slow, and ending
on those would report a ceiling of zero.

That number is the evidence that decides whether the WebGL2 batcher is worth
writing, and it has now been produced.

**The ceiling is about 34,000 spawned sprites at 60 fps**, on Canvas2D, on the
machine this was built on. Two runs: 33,627, then 34,476 with 9,045 of them
drawn — a spread of 2.5 per cent, which is close enough to call the number
repeatable. That is far past what the follow-up was written to protect
against, so **the WebGL2 batcher is not worth writing yet** — the evidence says
the renderer is not the thing standing in a game's way.

One thing qualifies the number, and it is the whole difference between the two
figures the lab now prints:

- **The ceiling counts entities; only a quarter of them are painted.** The
  world is 1280x768 behind a 640x360 canvas, which is 23.4 per cent of it by
  area, and the measured share is 26.2 per cent — the excess is the floor,
  always on screen and at most 252 tiles of it, plus the sprites the ramp has
  only just spawned, which start inside the view. Take the floor out and 8,793
  sprite draws carry a frame, from 34,476 that exist. **9,045 draws a frame is
  the renderer's number; 34,476 is the frame's.**

The ramp records `Stats.Drawn` where it stops, which is where the second
figure comes from; it used to be left to the reader, and the estimate this file
carried before it was measured was ten per cent low.

The other half is `Stats.SortMs`, which times `sortDrawOrder` apart from
`drawEntities`. `Engine.draw` used to start its clock before the sort, so the
number called *draw* was the sort plus the draw plus the game's own
`RenderUI` — and at the ceiling there are 35,437 entities to sort every frame,
around half a million comparisons, none of which is Canvas2D. The overlay's
*sort / draw* row now says which of the two a frame went on. **That reading has
not been taken yet**, and it is the one that would say whether a renderer swap
could reach the cost at all.

## Aseprite sheets — milestone 5, not started

Today every animation must be exactly 8 frames of exactly 100 ms in its own row.
Aseprite's own export carries named tags, real frame counts and a duration per
frame. `make sheet` already exists; the parser does not.

```sh
aseprite -b assets/lab.aseprite \
  --sheet web/static/img/lab.png --sheet-type rows \
  --data web/static/img/lab.json --format json-array --list-tags
```

The engine parses that into a `Sheet` of `Frame`s (rect plus duration) and `Tag`s
(name, from, to, direction — forward, reverse or ping-pong), and a game says
`e.Play(i, "walk")`. The old grid convention keeps working, because
`GridSheet(cols, rows, w, h, ms)` builds the same `Sheet` — one code path, two
ways in, so `game-jam-template` does not break. That equivalence is a pure host
test and is the backwards-compatibility proof.

**How to parse it — settled: a hand-written scanner.** TinyGo's own compatibility
guide names `encoding/json` as partially supported because it leans on
reflection, and recommends manual serialisation instead. Reported wasm cost is
around 119 KB for a reflective JSON path against about 27 KB without one — a 30
to 40 per cent regression on a 309 KB budget, to read six field names. Worse, the
failure mode is a runtime `reflect` panic in the browser with a green
`make check`. The scanner is about 250 lines, understands only
`frames[].frame.{x,y,w,h}`, `frames[].duration`, `meta.size.{w,h}` and
`meta.frameTags[].{name,from,to,direction}`, and skips every other value without
interpreting it — which is also what makes it survive Aseprite adding fields.

Limits, all checked and all fuzz targets: input at most 4 MiB, depth at most 64,
frames at most 65535, tags at most 1024, rects inside `meta.size` with positive
width and height, `0 <= from <= to < len(frames)`, durations clamped to
`[1, 60000]`, and `NaN` and infinities refused — JSON cannot spell them but
`1e400` parses to positive infinity.

**Aseprite is not installed on this machine**, so the export cannot be produced
here. The fixture must be a real export, made once on a machine that has
Aseprite, committed and never hand-edited — a hand-written one would only prove
the parser agrees with a guess about the format. Until it exists, ship a clearly
labelled placeholder and have the test `t.Skip` with a message saying so. Never
let a placeholder pass as evidence.

## Commands

The baseline Makefile, plus two rule-3 targets for the recurring commands the
gates cannot run. Targets alphabetical, `.DEFAULT_GOAL = check`, GNU Make 3.81
only.

```
make          every gate against the working tree
make ci       the same gates against the commit
make sheet    export every assets/*.aseprite to a PNG and its JSON
make wasm     tinygo build ./cmd/lab, wasm-opt, then check the size budget
make run      start cmd/serve on 127.0.0.1:8080
make test     go test -race -shuffle=on ./...
make build    release-shaped binaries in bin/
make fmt      goimports + go fix
make clean    rm -rf bin/
```

`check` names packages in its extra line —
`GOOS=js GOARCH=wasm go vet . ./cmd/lab/...` — rather than `./...`, because
`cmd/serve` has no build tag and vetting an HTTP server for js/wasm is noise.
Without that line the browser half is never checked at all on a developer's
machine.

`wasm` ends with a size gate against `WASM_MAX_BYTES = 320000`. A claim nothing
checks is a memory.

## Verification

Three tiers. All three tools are on this machine now, so all three tiers run.

**No extra tools — `make check`.** Green, the two network gates included. It proves the entity store, the camera, the state machine, the settings
defaults, the knob table's coverage, the menu's whole keyboard, the frame
statistics and the lab server. Four tests earn their place:

- **Every settings field has a knob**, checked with reflection on the host so it
  costs the wasm nothing.
- **The Go literal parses**, with `go/parser`, so the clipboard's promise is
  kept.
- **Measuring a frame allocates nothing**, via `AllocsPerRun`.
- **A fuzz target over the saved settings format**, because the browser's storage
  is user-editable.

The lab server was also run for real: correct CSP, `application/wasm`,
digest-based cache busting stamped into the page and every asset URL, `/static/`
answering 404, traversal refused, and both missing browser artefacts named at
boot with the command to run.

**With TinyGo installed — `make wasm`.** Done: 234,296 bytes, against a 320,000
gate that now actually runs, so the size claim cannot rot. Also settle the `ReadMemStats`
question before trusting the heap row, and use
`tinygo build -target wasm -opt=z -size=full -o /dev/null ./cmd/lab` to see which
package spends the bytes.

**In a browser — `make run`, then <http://127.0.0.1:8080/>.**

1. `M` — the menu opens over the running scene. Nudge
   `Feel.Heavy.ShakeMagnitude` with the right arrow, close it, press `3`, and see
   the difference without a rebuild.
2. `C` — paste the clipboard into an editor. It must be a Go literal that
   compiles.
3. Reload. The tuning is still there.
4. `H` — the overlay. `cpu / gpu load` must read `n/a`, not a number. Check
   Safari and Firefox too: what is being verified there is that the GPU name
   prints `(masked)` and the JS heap prints `n/a`.
5. Press `]` five times, then `0`. `go heap` should step up and **not** fall,
   because the entity slices keep their capacity. If it is a constant,
   `ReadMemStats` is not filling `HeapAlloc` under TinyGo — delete the row rather
   than ship it.
6. `B` — the ramp. Write down the sprite count where it stops. Done twice:
   33,627, then 34,476 with 9,045 drawn, which settles the WebGL2 decision for
   now. It adds 100 a second, so a ceiling that high takes about six minutes to
   reach. Still to read at the ceiling: the *sort / draw* row.
7. `F` — fullscreen. The overlay and the menu are still there, because they are
   drawn inside the canvas. Anything that disappears was in the DOM.

## Migrating `game-jam-template` — milestone 6, not started

Mostly mechanical, and worth doing in one commit so `make wasm` and the committed
`game.wasm` move together.

- Delete `internal/engine/`, add `require github.com/andygeiss/wisp-engine v0.x`.
- Create one `e := wisp.New(...)` in `main` and pass it down. `engine.EntityX[i]`
  becomes `e.X[i]`, `engine.StateEntity*` becomes `wisp.State*`,
  `engine.CanvasWidth` becomes `e.Width` — **and every surrounding `float64(...)`
  cast goes**, because `Width` and `Height` are already `float64`.
- `engine.Run(update)` plus `select {}` becomes `e.Run(update)`.
- Replace the six scattered hit-stop and shake literals with
  `e.Impact(e.Feel.Light)`, `.Medium` or `.Heavy`. `damageBoss`'s 4.0 and 150 and
  `hurtPlayer`'s 4.0 are already the defaults.
- Drop every hand-cleared key bool in favour of `e.Input.JustPressed(...)`. Five
  clear-sites go, and `stateAction2` stops being needed for edge tracking.

Six things are **not** mechanical:

- **The six `AddEntity` call sites** become `Sprite` literals. This is where a
  mistake hides: `imgCol` and `imgRow` are adjacent `int`s today. Read each one.
- **The hit-box fix changes collisions.** The box moves from 20x20 offset 6 px
  down-right to 20x20 centred. Play the boss fight before and after.
- **`handleAction1` and `handleAction3` are a judgement call.** They read `KeyQ`
  and `KeyR` level-triggered behind a cooldown, so holding the key re-fires the
  instant it ends. `JustPressed` is more correct and is a change a player will
  feel. Decide it; do not let the rename decide it.
- **`AnimationFrameDuration` is load-bearing.** The melee attack's hit window is
  written as frames 4 to 6, which only means 400 to 700 ms because a frame is
  100 ms. Moving to per-frame durations retimes every attack.
- **Two menus, one keyboard.** The game binds `P` and the engine binds `M`, so
  they do not collide — and because the engine swallows the keyboard while its
  menu is open, the game's menu freezes rather than double-stepping. Worth a
  sentence in the commit so nobody "fixes" it later.
- **`stateAction1` and friends start at bit 16.** Confirm the engine still claims
  only bits 0 to 13, so the game's bits stay free.

No longer a parking lot: `main.go` addresses the player as entity 0 throughout,
which used to mean deleting any entity below it silently repointed it. A delete
moves nothing now, so entity 0 stays entity 0 — see *Two changes networking
needs*. Worth checking the template for the opposite assumption instead: code
that iterates `for i := range e.State` now has to skip the holes with
`e.Live(i)`, and code that reads `e.Count()` as the iteration bound wants
`e.Slots()`.

## Networking — milestone 7, not started

A brief, and the measurements behind it. No code, and none until the first
question below is answered, because it is not a question about netcode.

### The brief

- **Job:** Two people play the same Wisp game in two browsers and see one
  world, with the server deciding what actually happened.
- **Why:** A game built on this engine today is one person at one keyboard.
  Making it two meant writing the netcode inside the game, where it could not
  reach the simulation it has to predict — and until the fixed tick landed, the
  simulation was not reproducible enough to predict at all. The two are the same
  problem: the engine owned the world and gave nobody else a way to agree with
  it.
- **Guardrails:**
  - Zero third-party dependencies, as everywhere else. That is what picks the
    transport rather than taste — see the table below.
  - `go list -deps .` on the root package keeps showing only the standard
    library. Netcode lives in subpackages the root does not import, and the
    server is a third `cmd/`, not a second root.
  - The client half compiles under TinyGo and stays inside the 320,000-byte
    gate. A probe put a `syscall/js` WebSocket with binary framing at 6.6 KB;
    treat 10 KB as the ceiling and fail the target above it, the way
    `WASM_MAX_BYTES` already fails.
  - The server never compiles under TinyGo and never imports the renderer.
  - `Settings` splits in two on the wire. `Feel`, `Camera`, `Render`, `Audio`
    and `Debug` are presentation and stay client-local and freely tunable.
    `World.Speed`, `World.HitBoxMargin` and `Time.TickRate` decide what happens
    and come from the server. Getting this wrong turns the engine's headline
    feature into a cheat menu.
  - The lab's measured path is not to be changed without re-measuring `B` and
    writing the new number down.
  - `game-jam-template` is not touched by this milestone.
- **Done means:**
  - Two browsers on one machine move each other's sprite, and the server is
    what decides where they are.
  - `go list -deps .` on the root package still shows only the standard library.
  - `make check`, `make ci` and `make wasm` are green, and the module is under
    the byte gate.
  - The RFC 6455 framing has its own tests: a masked frame, a fragmented
    message, a close handshake, and a frame that claims a length it does not
    have.
  - `SPEC.md` says this is part of the job, and anything this waives is in the
    README in the six-field form.

### The SPEC delta this asks for

`SPEC.md` is the project's own four fields, and this brief is a delta against
it — but a delta the current *Job* sentence does not cover, which is why it is
a decision and not a task. Concretely, three edits:

**Job** — one sentence becomes two, and the second is the new one:

> A solo developer tunes a 2D pixel-art browser game's feel in the browser, sees
> what it costs, and pastes the tuned numbers back into their own source.
> **When the game is for more than one person, the same engine runs on a server
> and decides what happened.**

**Guardrails** — three lines added, none of them replacing anything:

> - **The root package imports only the standard library, and only the standard
>   library.** `go list -deps .` is the check. The networking packages are
>   subpackages the root does not import, so a game that does not need them does
>   not pay for them.
> - **The transport is WebSocket, and that is a consequence rather than a
>   choice.** A browser has no UDP; WebRTC and WebTransport both need a
>   dependency or a server the standard library cannot be, so the dependency
>   rule picks this one. It is TCP, so the engine promises a 15-to-30 Hz
>   authoritative game and not a 60 Hz twitch shooter.
> - **`Settings` splits on the wire.** Presentation stays client-local and
>   tunable; `World` and `Time.TickRate` come from the server. The tuning menu
>   must not become a cheat menu.

**Done means** — one line added:

> - Two browsers on one machine move each other's sprite, the server decides
>   where they are, and `go list -deps .` still shows only the standard library.

Nothing in the current *Guardrails* or *Done means* has to go. The TinyGo rule,
the canvas-UI rule and the dependency rule all survive this milestone unchanged
— which is a good sign that it belongs here rather than in a second module.

### What the browser allows, measured rather than remembered

There is no UDP in a browser and no raw socket, so this is a choice between
three things. Probed with the installed TinyGo 0.42.0, `-opt=z` then
`wasm-opt -Oz`:

| Option | Cost | Verdict |
|---|---|---|
| **WebSocket through `syscall/js`** | 46,839 bytes against 40,232 for an empty module — **6.6 KB** | What we can afford, and the only one that keeps the dependency rule |
| `net/http` in the client | 2,895,075 bytes before `wasm-opt` — nine times the whole gate | Out on size, and `fetch`-shaped anyway |
| WebRTC data channels | the client is `syscall/js`; the **server** needs ICE, DTLS and SCTP | The only way to get real UDP semantics, and there is no standard-library anything. `pion/webrtc` is the dependency the guardrail exists to refuse |
| WebTransport | QUIC datagrams, technically the right answer | No standard-library QUIC server, and Safari |

The engine's own numbers for scale: `web/static/lab.wasm` is 234,882 bytes
against a 320,000 gate, so 85,118 bytes of headroom and the client half wants
about 8% of it.

**The consequence to write down, not to discover.** WebSocket is TCP, so one
lost packet stalls everything queued behind it. That buys a good 15 to 30 Hz
server-authoritative game and not a 60 Hz twitch shooter. For 2D pixel art it is
the right trade; it should be a promise the engine makes rather than something a
player finds.

The server side has no standard-library WebSocket either, but it does not need
one: RFC 6455 is a `sha1` and a `base64` for the handshake, `http.Hijacker` for
the connection, and a frame header of at most fourteen bytes. It is the same
move as the hand-written sheet scanner in *Aseprite sheets*, for the same
reason, and it costs the wasm module nothing because it never goes near it.

### Shape

```
wisp-engine/
├── net/          the wire format, imported by both halves. No syscall/js,
│                 no net/http: encoding only, so it tests on the host.
├── net/ws/       RFC 6455 framing. Server-side; the browser has its own.
├── net/client/   the syscall/js half (js && wasm)
└── cmd/gameserver/  the authoritative loop: one Engine, Tick on a ticker
```

The server runs the same `Engine`, on the same `Tick`, with `Simulate` set to
the same function — which is the whole point of the two changes that landed. It
holds no canvas, because `host.go` is already the headless twin and already
compiles everywhere.

### The decisions this brief does not settle

1. **Does the engine's job change, or does this become a second module?**
   `SPEC.md` says the job is tuning a game's feel in the browser. Networking is
   not a delta against that sentence, it is a different sentence, and the
   baseline calls a second binary and a first external system a change of
   shape. The recommendation is subpackages here and a widened job, because
   client prediction has to snapshot and restore the entity store — a separate
   module would need the engine to export those hooks anyway, which is the
   coupling without the benefit. **This one has to be answered before any code.**
2. **Server-authoritative with client prediction, or deterministic lockstep?**
   Recommend authoritative: lockstep needs bit-identical floats across two
   browsers, which `math.Sqrt` and `math.Exp` do not promise.
3. **What tick rate goes on the wire?** Recommend 30, and let the client keep
   rendering at its own frame rate. Needs the interpolation above first.
4. **`randFloat` is still the unseeded global `math/rand/v2`.** `random.go`
   already says it is one function so *"a build that needs a seeded one has one
   place to change"*. Two machines cannot agree until it is seeded per match —
   and the shake has to stay outside that stream, because presentation must not
   be able to desync a simulation.

## What this does not do

- **No Aseprite sheets yet.** Milestone 5, designed above, not written.
- **No native build.** Real OS-level CPU, RAM and GPU numbers need a second
  platform layer. A follow-up, and a large one.
- **No WebGL2 renderer.** A follow-up, gated on the number `B` produces.
- **No interpolation between ticks.** The simulation is fixed-rate and the draw
  reads the current position, so a frame that runs two ticks or none shows a
  one-tick pop — about two pixels at the default speed. It wants a previous
  position per entity and a lerp in the draw. Until it exists the lab's own
  bounce stays in the per-frame update, so the showcase does not judder.
- **No networking.** Milestone 7 is a brief and two pieces of groundwork. The
  transport is decided by the dependency rule and measured; nothing is written.
- **No particles, tweens, tint, rotation or squash-and-stretch.** None of them
  are representable — `drawImage` is called with source size equal to destination
  size and there are no scale, rotation or tint arrays. They arrive with the
  WebGL2 renderer, where the shader gives them almost for free.
- **No `devicePixelRatio` handling**, deliberately. A fixed 640x360 backing store
  is what makes the lab's frame times comparable between machines.
- **`game-jam-template` is untouched** and still carries its own copy of the
  engine.

## Baseline rules waived, on the record

Four, in the README in the six-field format.

1. **No `main` package** (`project-types/library.md`) — the lab is the engine's
   own proof and has to be runnable from one clone. Scoped to `cmd/lab` (js/wasm
   only) and `cmd/serve`; the library imports neither, and `go list -deps .`
   shows only the standard library. Nothing is embedded, so the second half of
   that rule needs no waiver.
2. **No hand-written JavaScript** (`stack/html.md`) — a browser can only start a
   WASM module from a script, and the module's own size and the GPU's name are
   not reachable from Go. Scoped to `web/static/js/`: TinyGo's `wasm_exec.js`,
   copied and never edited, and a 119-line loader.
3. **CSP `script-src` carries `'wasm-unsafe-eval'`**
   (`patterns/security-headers.md`) — browsers refuse to compile WebAssembly
   without it. `'unsafe-eval'` and `'unsafe-inline'` stay banned, and a test pins
   that neither appears as a token.
4. **`make check` has two lines the baseline's does not** (`stack/makefile.md`
   rule 1) — the js/wasm vet, and the size check on the built module.

Conformance notes worth repeating here:

- **Nothing is embedded, and that is the rule rather than a deviation.** A
  library ships no `main` package, no embedded assets and no CLI
  (`project-types/library.md`); the layout pattern that *does* want a binary
  carrying its own `web/` says in its opening line that it is for a web
  application. Reading the tree from disk is compliance twice over, so the
  fifth waiver this looked like it needed does not exist.
- **The version is a digest of the served tree, not `debug.ReadBuildInfo`.** The
  assets are on disk, so the binary's identity says nothing about them. Hashing
  what is served is what the baseline's real rule — two builds with different
  assets must not share a version string — is asking for.
- **`context.Context` is not the first parameter of `Engine.Run`**, the only call
  that blocks. The browser owns the frame loop's lifetime and nothing on the Go
  side can cancel it; `Engine.Stop` is the handle the rule wants.
- **The library checklist's first box is still unchecked.** *A second project
  actually imports this* is what milestone 6 is for. Until then this is an
  extraction with one consumer, which is the shape the rule exists to catch.
