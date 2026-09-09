# Extract the Wisp engine into a standalone, tunable module

**Status: milestones 1 to 4 are built and green. 5 and 6 are not started.**
This file is both the plan and the record — what was decided, what was built,
what changed while building it, and what is left.

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
  TinyGo 0.42 — 309 KB is the number to protect; the module measures 228 KB. All overlay UI is drawn on the
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

Two follow-ups, each gated on evidence rather than scheduled:

- **A WebGL2 batched renderer**, if and when the number `B` produces is too low.
- **A native backend** for true OS-level CPU, RAM and GPU numbers. A whole
  second platform layer, and a large one.

### Gates

Green, all of them: `gofmt`, `go vet`, `GOOS=js GOARCH=wasm go vet`,
`go fix -diff`, `staticcheck`, `govulncheck`, `go mod tidy -diff`,
`go test -race -shuffle=on`, `CGO_ENABLED=0 go build`. `make ci` runs that same
list against the commit and is green too. 43 test functions: 38 tests, four
examples, and one fuzz target with its seeds.

`make wasm` has run. `web/static/lab.wasm` is committed at 233,832 bytes — 228
KB, under the 320,000-byte gate — built with TinyGo 0.42.0 and LLVM 22.1.4.

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
│   ├── lab/main.go      290 the playground (js && wasm)
│   └── serve/main.go    230 the static file server behind `make run`
├── DESIGN.md                the page's tokens
├── doc.go                54 the package doc
├── engine.go            321 Engine, Config, New, Step, Impact, HitStop, Shake
├── entity.go            215 Sprite, Tilemap, Add, Delete, BoundingBox, Reset
├── example_test.go       72 runnable Examples for pkg.go.dev
├── go.mod                   go 1.27, no requires
├── host.go              115 the same API, headless (!js || !wasm)
├── input.go             129 key state, edge detection, the menu's lock
├── internal_test.go     437 draw order, layering, menu widths, the frame ring
├── knobs.go             321 the knob table, GoLiteral, the text format
├── LICENSE                  MIT
├── Makefile                 baseline Makefile + sheets and wasm targets
├── menu.go              352 the M overlay, canvas-drawn
├── metrics.go           314 the frame ring, percentiles, the H overlay
├── PLAN.md                  this file
├── random.go              8 the engine's one source of randomness
├── README.md                install, the 30-second example, waived rules
├── render.go            111 Run, Stop, Rect, Text, sound, the frame's paint
├── runtime_js.go        464 canvas, events, audio, rAF loop (js && wasm)
├── settings.go          203 Settings and its eight groups, Defaults
├── settings_test.go     244 the literal, the coverage net, the fuzz target
├── SPEC.md                  job, why, guardrails, done means
├── state.go             191 the state bits, movement, animation, draw order
├── web/
│   ├── static/              app.css, wasm_app.js, the art, lab.wasm
│   └── templates/index.html the host page
└── wisp_test.go         833 the consumer's view: entities, camera, input, menu
```

2,359 lines of engine that build anywhere, 579 behind the browser tag, 1,765 of
tests, 520 of lab and server.

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

Five, from installing TinyGo and opening the page for the first time. Every
one of them was invisible to `make check`, because a green gate says nothing
about a tree nobody serves, a scene nobody looks at, a line of text nobody
measures, or a number nobody reads.

| Fix | What happened |
|---|---|
| `GET /static/js/wasm_exec.js` answered 404, and `make wasm` had never produced anything. `sheets` and `wasm` wrote under `cmd/serve/web/static/`, a directory that does not exist — so the target died on its first `cp`, silently, every time. | Both targets now write to `web/`, the tree `cmd/serve` actually reads and where the committed art already lived. It is also where the baseline's `go-project-layout.md` puts it. |
| `WASM_MAX_BYTES` was declared and never read. Its own comment called it "the headline number, as a gate", and the `wasm` target's said "the size check is last" — but nothing compared anything. | The target's last line is the comparison. A claim nothing checks is a memory, including this one. |
| **The menu's hint line was cut off at the right border**, showing "T te" for "T test". | The panel is pinned to the canvas's right edge and its text is left-aligned inside it, so a line wider than the panel is not clipped to the panel — it is drawn past the edge of the canvas and its tail is gone. The hint was 32 characters where 29 fit. `menuCols` now states the budget with its arithmetic, the hint is `←→ tune  ⇧ x10  C copy  T hit`, and `TestMenuTextFits` checks every fixed string, every knob's footer and every knob row against it, counting runes rather than bytes — `←` is three bytes and one column. |
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

## `Settings` — the point of the project

Eight groups, alphabetical, every field a number or a bool so one menu can edit
all of them. **38 knobs across 11 nesting levels**, because `Feel` holds four
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
writing. TinyGo and a browser are no longer the blocker, and neither is the
ramp: it used to stop on its first check at zero sprites and show nothing,
because it compared the 99th percentile against the middle of the window. It
now stops on a frame the display actually missed. **The number itself has
still not been written down.**

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

**With TinyGo installed — `make wasm`.** Done: 233,832 bytes, against a 320,000
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
6. `B` — the ramp. Write down the sprite count where it stops. That is the
   Canvas2D ceiling, and the WebGL2 decision.
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

Parking lot, and the engine's `Delete` doc comment says so: `main.go` addresses
the player as entity 0 throughout, so deleting any entity below it would silently
repoint it.

## What this does not do

- **No Aseprite sheets yet.** Milestone 5, designed above, not written.
- **No native build.** Real OS-level CPU, RAM and GPU numbers need a second
  platform layer. A follow-up, and a large one.
- **No WebGL2 renderer.** A follow-up, gated on the number `B` produces.
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
