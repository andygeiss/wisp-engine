# Extract the Wisp engine into a standalone, tunable module

**Status: milestones 1 to 6 are built and green, and 7 is done: two tabs on
one machine play one world, which Andy confirmed in a browser on 2026-09-09;
the numbers step 9 asked for were measured the same day and are written down
under that milestone's *Verification*; and `v0.2.0` is the annotated tag on
the commit that writes them. The blend has since been measured in a real
Chrome, frame by frame, and does what it claims — *The blend, measured* has
the numbers — and the first look at the two overlays found two lines of text
that did not fit, fixed the same day, and one question that is a person's:
whether a floor tile's hit box belongs on the overlay. Still open, and still
a person's: the eye's own verdict on the blend, the tile question, and the
ruling on the client half, which missed its size ceiling by a measured margin
that is the brief's owner's to rule on; *What is left*, near the end, lists
every open item with who closes it.** This file is both the plan and the record — what was decided, what was
built, what changed while building it, and what is left.

**Milestone 8 is decided, and built.** PixelLab is the art source: a
manifest of PixelLab IDs under `assets/`, one make target that turns it into
the art the engine serves, an engine that faces eight ways, a floor that
autotiles, and a lab drawn entirely from generated art — a ranger that
walks, idles and strikes in eight directions, a crowd of foxes that walk
where they bounce, a lake with a shore, a dirt path and four props, for 46
generations. Every gate is green, the module is 278,777 bytes, a headless
Chrome drew both scenes, and Andy played it on 2026-09-10 and said it
works; `v0.3.0` is the annotated tag on the commit that records it. The
analysis of 2026-09-10
— a probe tileset, the measured export formats and the four decisions Andy
took on them — is what it built on; *PixelLab — milestone 8* is the brief,
the decisions and the record. The server that ships, which this file used to
call milestone 8, is milestone 9 now and still waits for a brief of its own.

**Milestone 7 is decided, and built.** The lab becomes the network client and the server
that serves it runs the world: the game state lives on the server, the browser
sends only what the player is trying to do, cooldowns live on the server and
the client copies them for its bars, and the camera is the client's. That is a
thin client, and it dissolves three of the four questions the old brief left
open. *Networking — milestone 7, done* is the plan and the record of
building it, and `SPEC.md` carries the wider job as of the same change.

**Three changes landed ahead of it**: an entity index is now a name the engine
keeps, the simulation runs on a fixed tick instead of on however long the frame
took, and the draw blends between two ticks so that the tick is not something a
player can see. All three stand on their own merit and all three are green —
*Three changes networking needs* has them.

**The module has a consumer.** `game-jam-template` deleted its copy of the
engine and requires `github.com/andygeiss/wisp-engine v0.1.0` — the repository
is public and tagged, both gates are green on the commit, and the library
checklist's first box is checked at last. *Migrating `game-jam-template`* is
the record of what was not mechanical about it.

**The lab has since been built and run.** TinyGo and `wasm-opt` are installed,
`make wasm` produces a 268 KB module, every gate is green including the two
that need the network, and the scene has rendered in a browser. That first run
is what turned up the floor-layer bug in *Fixes on the first real run*.

**The blend between two ticks has now been measured, and it does what it
claims.** It was the loose end everything after it leaned on: the claim is
that the motion is visibly smoother, which is a claim about `pass` in
`runtime_js.go`, the one line it changed that no test reaches, and there was
no browser to look with, so the client was built on it unseen. On 2026-09-10
a headless Chrome was driven over its debugging pipe instead: it holds the
page at a steady 60 frames a second, holds a key down, and reads the canvas
back every frame. With the knob on, the floor scrolls an even 2 px a frame
behind a walking player; with it off, it moves 0, 0, 6 px at a 20 Hz tick and
4, 0, 4, 0 at the server's 30 — the tick made visible, exactly as *The draw
blends between two ticks* said it would be. *The blend, measured* has the
table and how it was taken. What a machine cannot say is whether it *looks*
right to a person, so step 8 of *Verification* keeps one line for Andy's eye.

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
  TinyGo 0.42; the module measures 268 KB against a 320,000-byte gate. All
  overlay UI is drawn on the canvas, never as DOM, so it survives fullscreen.
  309 KB was the number to protect in `game-jam-template`, and it was not
  protected: importing the module took that game to 411 KB. See *Migrating
  `game-jam-template`*, which says where the bytes went and what they buy.
- **Done means, and all four now are:** `make check` and `make ci` are green;
  the library checklist is walked with every box checked or waived in the
  README; the lab runs in a browser and `M`, the spawn key and the overlay all
  do what they say; `game-jam-template` imports the module and still plays.
  The last of those closed with milestone 6 — the checklist's first box could
  not be checked by this repository at all, only by a second one importing it.

## Where it stands

| Milestone | State |
|---|---|
| 1. The module, the `*Engine` struct, the migrated tests | **done** |
| 2. Something to open: `cmd/lab`, `cmd/serve`, the host page | **done** |
| 3. `Settings` and the `M` menu | **done** |
| 4. The metrics overlay and the sprite spawner | **done** |
| 5. Aseprite sheets | **done** — and Aseprite turned out to be installed after all |
| 6. `game-jam-template` imports the module | **done** — and it cost the game 100 KB |
| 7. The lab plays over the wire | **done** — two tabs on one machine, confirmed in a browser on 2026-09-09; the numbers of step 9 measured with a bot the same day and written into its *Verification*; tagged `v0.2.0`. The blend of step 0 was measured in a headless Chrome on 2026-09-10 and holds; the brief, the decisions and the order are below |
| 8. PixelLab is the art source | **done** — decided and built 2026-09-10, for 46 generations; every gate green, 278,777 bytes; drawn in a headless Chrome and played by Andy the same day; tagged `v0.3.0`. *PixelLab — milestone 8* is the brief, the decisions and the record |

Three follow-ups, each gated on evidence rather than scheduled:

- **A `wisp_release` build tag** over `menu.go`, `knobs.go`, `metrics.go` and
  the settings text format, so a shipped game strips what only a developer
  looks at. Milestone 6 produced the evidence: importing the module cost
  `game-jam-template` about 100 KB, and a `strings` scan of the build says all
  of it is the tuning menu, the settings format and the overlay. It splits the
  code path in two and every gate would have to run both ways, which is why it
  is a milestone and not a tidy-up.
- **A WebGL2 batched renderer**, if and when the number `B` produces is too low.
  It is not: `B` reports about 34,000 sprites at 60 fps over two runs, 9,045 of
  them actually drawn. The overlay's *sort / draw* row splits the frame, so a
  rerun says how much of it a new renderer could even reach.
- **A native backend** for true OS-level CPU, RAM and GPU numbers. A whole
  second platform layer, and a large one.

And one loose end that was not a follow-up but a defect, now closed:
**`Debug.ShowHitBoxes` drew nothing.** The knob was in the table and in the
menu, and its doc comment said it "draws every entity's collision box" —
nothing anywhere read it. It turned up while working out which readers of a
position wanted the blend and which wanted `e.X[i]`: a hit box wants `e.X[i]`,
and then there was no hit box to want it. It was either a few lines against
`BoundingBox` or a knob that should go, and until it was one of the two it was
a claim nothing checked — the same lesson as `WASM_MAX_BYTES` in *Fixes on the
first real run*, found the same way. It is the few lines now. `eachHitBox` in
`render.go` lists the boxes of one pass, culled the way the sprites are, and
each backend draws that list: the browser as one-pixel `strokeRect` outlines
over the sprites of the pass, the headless twin into a record a test reads
back. The box sits at `e.X[i]`, so with the blend on it runs up to a tick
ahead of the sprite — the blend made visible, not a defect. An invisible
entity's box is drawn too, because an unseen collider is what somebody
switching the knob on is looking for; a box of zero size is not, because it
never collides. `TestShowHitBoxesDrawsWhereTheRulesAre` pins all of that, in
`host_test.go` under the host build tag, because the js vet compiles the test
files too and only the headless backend keeps the record. It cost 1,607
bytes. **It has had its look, on 2026-09-10 in a headless Chrome**, magnified
five times: standing still, the box is centred on the sprite; walking with
the blend on, it runs a few pixels ahead of the sprite, as the doc comment
says; with the blend off the two coincide; and every bouncer's box sits on
its bouncer, leading it a little the way it is moving. What the look also
found is that a floor is 960 entities and every one of them has a 20x20 box:
with the knob on, the lab's screen is 253 red squares in a grid and the
player's is one more of them. That is what the doc comment promises — "every
entity's hit box" — and `AddTilemap`'s tiles are entities with sprites, so
`BoundingBox` and `HasCollision` are as true of a tile as of anything.
Whether a debug overlay should still draw them is a design question and it
is not settled here: skipping them needs a way to tell a tile from a still
sprite, which the engine does not have — a marker bit would be the last
spare, 15 — and a game that uses tiles as walls would want their boxes
shown. *What is left* names it.

### Gates

Green, all of them: `gofmt`, `go vet`, `GOOS=js GOARCH=wasm go vet`,
`go fix -diff`, `staticcheck`, `govulncheck`, `go mod tidy -diff`,
`go test -race -shuffle=on`, `CGO_ENABLED=0 go build`. `make ci` runs that same
list against the commit and is green too. 156 test functions across eight
packages: 147 tests, 5 examples, and 4 fuzz targets with their seeds —
`FuzzParseSheet`, `FuzzUnmarshalText`, `FuzzDecode` and `FuzzRead`.
`FuzzParseSheet` has also been
run for 45 seconds — 10.9 million executions, no crash and no sheet that
violated an invariant the draw trusts.

`make wasm` has run. `web/static/lab.wasm` is committed at 278,777 bytes — 272
KB, under the 320,000-byte gate — built with TinyGo 0.42.0 and LLVM 22.1.4.
Milestone 8 put 4,346 on: 168 for the facing, and 4,178 for the lab's art —
the direction table, the two sheets, the autotiler and the level, the
bouncers turning their sprite, the strike's bit. The
free list and the fixed tick cost 586 bytes of that between them, the sheet
2,615 more, and the blend between two ticks 1,972; the three hooks of
milestone 7 then took 36 off, because folding the movement into one function
gave the compiler back more than the new branch cost, the hit-box overlay put
1,607 on, moving the solo scene onto `internal/lab` 1,041 more, the
network client 31,736 — *The client is built* has that one piece by piece —
and the two overlay lines that did not fit 614, of which 395 is unwrapping
the GPU's name.
`ParseSheet` is not
among those 2,615: the lab builds its sheet with `GridSheet`, so the scanner is
dead code the linker drops.

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
| Metrics | Honest browser proxies only. CPU% and GPU% read `n/a — no browser API`, never a made-up number. |
| Sheet parsing | A hand-written scanner, not `encoding/json`. See *Aseprite sheets*. |
| Distribution | Public, and tagged. `game-jam-template` is a public repository, so a private module behind it would have been a template nobody but its author could build. `v0.1.0` is the first tag; its message lists the API it pins, because *Job* says the tag message is where a break is written down. `v0.2.0` is the second: it names the three hooks and the blend's API, and says what broke — the blend is on by default, so an entity moved from the per-frame update is drawn dragged back toward the tick until that movement moves into `Simulate`; bit 14 is `StateRemote` and no longer spare; and `Debug.ShowHitBoxes` draws now. |
| Networking | A thin client. The server runs the world and the browser sends intent — no prediction, no lockstep. Three of the old brief's questions dissolve with it; see *Networking — milestone 7, done*. |
| Transport | WebSocket, hand-written on both sides, because the dependency rule refuses everything else a browser offers. TCP, so a 15-to-30 Hz game; 30 on the wire. |
| Where the netcode lives | `internal/`, public the day a second consumer needs it. `cmd/serve` grows into the game server; there is no third `cmd/`. |
| The lab's skills | `Q` strike, `E` spawn, `R` dash — the template's keys and cooldowns, so its migration maps one to one. |
| Encoding on the client | Through each message's own `Append`, never through the `wire.Message` interface: the second concrete type to reach one interface call cost the module 11.6 KB of dispatch, measured. |
| The art | PixelLab, from a manifest of IDs; the `.aseprite` sources and `make sheets` go. `make art` downloads by ID, no token, and is the only way art enters the tree. Andy, 2026-09-10. |
| The art tool | `cmd/art`, a third `main`, host-only: it crops character sheets and writes what the engine reads. The no-`main` waiver widens to it. Andy, 2026-09-10. |
| Facing | `Engine.Facing`, a field next to `RowMask`: 2 by default, which is today's behaviour, or 4 or 8. Andy, 2026-09-10. |

## What the repository holds

```
wisp-engine/
├── assets/pixellab.txt      the art, by PixelLab ID: kind, ID and name, one a line
├── camera.go            144 follow, dead zone, look-ahead, bounds, shake
├── cmd/
│   ├── art/main.go         403 the art tool: the manifest, the downloads, the sort, the crop
│   ├── art/main_test.go    331 against a stand-in server: the files, a second run, a 423
│   ├── lab/main.go         71 the entry point: the images, the rules, solo or the wire
│   ├── lab/net.go         229 network mode: intents out, the replica in, the HUD
│   ├── lab/socket.go       95 the one syscall/js file outside the engine
│   ├── lab/solo.go        207 today's scene: the spawner, the ramp, B
│   ├── serve/client.go      252 one socket: its reader, its writer, the hub
│   ├── serve/config.go      103 the flags, each defaulting to its variable
│   ├── serve/config_test.go  92 defaults, precedence, every refusal
│   ├── serve/main.go        112 the wiring: config, world, server, shutdown
│   ├── serve/main_test.go   183 headers, caching, the version, traversal
│   ├── serve/pages.go       181 the page and the tree, the static half
│   ├── serve/world.go       292 the tick: joins, intents, skills, the snapshot
│   └── serve/world_test.go  582 two players over net.Pipe, in a synctest bubble
├── DESIGN.md                the page's tokens
├── doc.go               111 the package doc
├── engine.go            450 Engine, Config, New, Step, Tick, DrawPos, Impact
├── entity.go            336 Sprite, Tilemap, ID, Add, Delete, Place, Slots
├── example_test.go       75 runnable Examples for pkg.go.dev
├── go.mod                   go 1.27, no requires
├── host.go              127 the same API, headless (!js || !wasm)
├── host_test.go          48 what the headless twin recorded, read back
├── input.go             133 key state, edge detection, the menu's lock
├── internal/
│   ├── lab/art.go         244 the images, the directions, the two sheets, the rows, the props
│   ├── lab/art_test.go    143 the constants against the committed JSON and PNGs
│   ├── lab/floor.go       120 the Wang table, the autotiler, the lake and the path
│   ├── lab/lab.go         333 the rules both halves share: skills, the strike's bit, bouncers
│   ├── lab/lab_test.go    471 the cooldown gate, the strike and its swing, the cap, the dash, the bounce, the floor
│   ├── pixellab/pixellab.go 302 the layout and the metadata as Go, the Wang table, the row order, the crop
│   ├── pixellab/pixellab_test.go 232 on real exports: the mage's sheet and the probe's metadata
│   ├── replica/replica.go 218 the server's world in an engine: slots, queue, the depth rule
│   ├── replica/replica_test.go 289 apply, hold, fast-forward, catch-up, order, the blend's pair
│   ├── wire/wire.go       440 the ten messages and their bytes
│   ├── wire/wire_test.go  197 round trips, the four refusals, the fuzz target
│   ├── ws/frame_test.go    64 the RFC's key, and the fuzz target over the reader
│   ├── ws/ws.go           511 RFC 6455: Accept, Dial, frames, close, ping
│   └── ws/ws_test.go      521 every rule the brief lists, over net.Pipe
├── internal_test.go     780 draw order, layering, frame mapping, source rects, the two overlays' text
├── knobs.go             323 the knob table, GoLiteral, the text format
├── LICENSE                  MIT
├── Makefile                 baseline Makefile + sheets and wasm targets
├── menu.go              358 the M overlay, canvas-drawn
├── metrics.go           449 the frame ring, percentiles, the H overlay
├── PLAN.md                  this file
├── random.go              8 the engine's one source of randomness
├── README.md                install, the 30-second example, waived rules
├── render.go            148 Run, Stop, Rect, Text, sound, the frame's paint
├── runtime_js.go        491 canvas, events, audio, rAF loop (js && wasm)
├── settings.go          228 Settings and its eight groups, Defaults
├── settings_test.go     244 the literal, the coverage net, the fuzz target
├── sheet.go             270 Sheet, Frame, Tag, GridSheet, Play, the frame map
├── sheet_json.go        586 the hand-written Aseprite scanner and its limits
├── sheet_test.go        501 the export, the grid equivalence, the fuzz target
├── SPEC.md                  job, why, guardrails, done means
├── state.go             226 the state bits, movement, animation, draw order
├── testdata/aseprite/       the old Aseprite export, as the parser's fixture
├── web/
│   ├── static/              app.css, wasm_app.js, lab.wasm, and img/: what make art wrote
│   └── templates/index.html the host page
└── wisp_test.go        1458 the consumer's view: entities, camera, input, menu
```

3,897 lines of engine that build anywhere, 491 behind the browser tag and
3,106 of tests; 1,542 of lab and server with 857 of tests; 1,551 under
`internal/` with 1,318 of tests. The counts in that listing are hand-written and
have now been wrong four times — most recently `doc.go`, which grew eight lines
with the blend and kept its old number. `wc -l *.go cmd/*/main.go` is how they
were put back, and is what to run rather than trust them.

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

## Three changes networking needs

All three landed before milestone 7 was decided, because all three are worth
having whether or not it happens. None of them is network code; all of them are
things the engine got wrong for a game that is only ever played by one person
at one keyboard.

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
from the server rather than from a saved menu. Milestone 7 makes that concrete: the
rate arrives in the server's Welcome message, and the client writes it over the
knob every frame, because a saved menu may still hold the 20 from step 8's
experiment.

**What this left visible is the next section.** At 60 ticks on a 60 Hz display
the frame jitter means the occasional frame runs two ticks or none, which was a
one-tick pop — about two pixels at the default speed, and a third of the frames
on a 144 Hz display. Pixel-art snapping hid most of it and none of it was
wrong, but a fixed tick is a thing a player could see, which is not what a
fixed tick is for.

**The ramp number needs re-reading.** `updateStates` moved from every frame to
every tick, so the per-frame cost changed. It should not have changed much —
the spawned sprites carry no move bits and no `RowMask` bits, so that loop was
already skipping them, and `advanceAnimations`, the pass that does touch all of
them, is still per frame. But 34,476 was measured against the old shape and is
now a number about a build that no longer exists.

### The draw blends between two ticks

`Step` spends a frame's world time on whole ticks and carries the remainder, so
a frame runs two ticks, or one, or none — and the draw read `e.X[i]`, which is
where the *simulation* is. A frame that ran no tick therefore drew the previous
frame's pixel and the next one jumped two ticks' worth.

`Tick` now keeps the position it is moving away from — `PrevX` and `PrevY`, two
slice copies at the top of it — and `DrawPos(i)` returns the two blended by how
far through the current tick the frame sits.

| Reads the blend, because it is what somebody sees | Reads `e.X[i]`, because it is what the rules are about |
|---|---|
| `pass`, the one hot loop in `runtime_js.go` | `BoundingBox`, `HasCollision`, and the hit-box overlay drawn from them |
| `follow`, so the camera and the sprite it is following agree | `sortDrawOrder` and the viewport cull |
| a game's own `RenderUI` — a health bar over a monster | `updateStates`, `Simulate`, every rule |

The camera is the half of that table worth explaining. `Camera.Smoothing`
defaults to 0, which is a hard snap to the target: a camera snapping to the
tick position while the sprite it follows draws at the blended one would leave
that sprite jittering against the screen — the judder moved rather than fixed.

**It costs up to one tick of lag, and that is the trade rather than a defect.**
The blend is between two positions the simulation has already been. The other
way is to guess forward from the last one, and a guess is wrong every time
something turns around; a sprite that overshoots a wall and snaps back is worse
than a sprite a sixtieth of a second behind.

Three things had to follow, and each is a test that fails without it:

- **`Add` starts an entity where it is.** `set` writes `PrevX` along with `X`,
  so a new entity does not streak in from the origin — and, because `Delete`
  empties a slot through the same function, a reused slot does not streak in
  from whoever had it last.
- **`Place(i, x, y)` is how a game jumps an entity**: a respawn, a teleport, the
  far side of a door. The draw cannot tell a jump from a very fast tick, so
  without it the jump is smeared across one.
- **The lab's bounce moved into `Simulate`.** This file said why it could not
  before: moving it there ahead of interpolation would have put the judder into
  the showcase. The reverse is now true. An entity moved once a frame is
  already exactly where that frame wants it, so blending drags it *backwards*
  toward a tick it never took — which turns "movement belongs in the tick" from
  advice into a rule with teeth.

**`Render.Interpolate` is the 40th knob, default on.** It is presentation, so
milestone 7's guardrail leaves it client-local and freely tunable. Off, every
sprite sits exactly where the simulation put it and moves in the steps the
simulation moved it in, which is what the engine did before — one keystroke
away for anybody whose game it does not suit.

The previous position is kept whether the knob is on or off. It is two slice
copies a tick against a per-drawn-entity cost in the draw that the knob does
turn off, and keeping it means a knob switched on mid-scene cannot blend from
wherever the world was when it was switched off.

**It cost 1,972 bytes**, 237,497 to 239,469 against the 320,000 gate. What it
did not cost is a per-entity array of anything but the two floats: no new
branch in `updateStates`, `advanceAnimations` or the draw's cull, because the
blend is arithmetic on two numbers that were already there.

**`game-jam-template` has to move its movement before it upgrades**, and this
is the one consequence that reaches outside this repository. It sets no
`Simulate` at all: the monsters chase the hero and the hero is clamped to the
world from the per-frame update, which was the only place there was when that
code was written. Those entities are moved every frame and drawn every frame,
so today they are perfectly smooth — and a blend would drag them back on the
frames that carry no tick, which is the artifact this change removes from
engine-moved entities arriving at game-moved ones. It is pinned to `v0.1.0`, so
nothing is broken now; whoever bumps it moves that movement into `Simulate`
first, which is where the fixed tick wanted it anyway. Its hero already
benefits either way, because the engine's own move bits have run on the tick
since the tick existed.

**Nobody had looked at it until 2026-09-10.** Everything above is an argument
about arithmetic, and the tests hold that half of it: four of them fail if the
blend is stubbed out, three more if `set` or `Place` stops writing the previous
position, and both were run against a broken build rather than assumed. But
"the motion is visibly smoother" is not something a test can say, and the only
line changed in the draw is one that only the js build ever runs — which is the
same shape as milestone 5's switch to a sheet, where every argument was about
`srcRect` and the browser was what checked it. Step 8 of *Verification* is that
check for this one, and the next section is how it was done with nobody at the
keyboard.

### The blend, measured

Smoother is a claim about how far the picture moves from one frame to the
next, and a browser can be asked that. A headless Chrome 152 was started with
`--remote-debugging-pipe` and driven over DevTools from a script in the
session's scratch directory — Python's standard library, no module — because
the Chrome extension was not connected, for the third session running, and
Chrome's `--screenshot` flag catches one frame and leaves. Driven this way the
page runs for real: `requestAnimationFrame` came every 16.7 ms, never under
16.5 or over 16.8, on the machine's own GPU, and a `KeyboardEvent` dispatched
on `window` without its `keyup` is a key held down, which is what walking
needs. The knobs were seeded through `localStorage` before the module loaded,
in the menu's own `Group.Field=value` lines, so no menu had to be steered.

The sampler is thirty lines of JavaScript run inside the page: every
animation frame it reads a band of the canvas back with `getImageData`,
compares it with the previous frame's, and records two numbers — how much the
picture changed at all, and the sideways shift that best explains the floor.
The floor is what moves when the player walks, because the camera snaps to
the player's *drawn* position and so the player stays put on screen; the
bouncers are what move when the player stands still. Those are the two routes
the check asked for — the engine's own move bits and `Simulate` — one seen
through the camera and one directly. Three seconds each, 180 frames, `?solo`
at `Time.TickRate` 20 and the wire at the server's 30, the blend on and off:

| | blend on | blend off |
|---|---|---|
| `?solo`, tick 20, walking: the floor's shift per frame | 2 px on every frame and 3 on about one in ten, never 0 — 2.08 on average, which is 6.25 px a tick spread over three frames | 0, 0, 6, then 0, 0, 7: 71 of 107 frames moved nothing and the rest moved 6 or 7 px |
| `?solo`, tick 20, 100 bouncers, standing still: frames on which the picture did not change | 0 of 180 | 96 of 180, in runs of two |
| the wire, tick 30, walking: the floor's shift per frame | 2 px on every frame and 3 on about one in nine — 2.08 on average, which is 4.17 px a tick over two frames | 4, 0, 4, 0, then a 5: 52 of 105 frames moved nothing |
| the wire, tick 30, the server's crowd of 100, standing still: frames on which the picture did not change | 0 of 180 | 79 of 180, every other frame |

Both walks ran into the world's edge after about 107 frames, where the camera
stops and every later frame moves nothing; the counts above are the frames
before it. The wire's walk with the blend on held once, for two frames, right
after its first snapshot — the jitter buffer's "an empty queue holds for a
tick", seen at the join and never again, and the HUD's `held` read 1 at the
end. The frames on which the picture did not change with the blend off are
the tick made visible: at 20 ticks on a 60 Hz display two frames in three
draw the same picture and the third jumps a whole tick, at 30 every other one
does, and with the blend on there is no such frame. Only the animation's own
frame changes, every 100 ms, show through either way, which is presentation
and runs per frame by design.

That is the verdict the arithmetic promised, taken from the committed module
in a real Chrome, and it is what a network client draws. What it is not is
an eye: whether 2 px a frame at 60 Hz *reads* as smooth on a display in
front of a person is Andy's to say, and step 8 keeps that line. The driver
was not kept — a Python script has no place in this tree — and the recipe is
here so it can be written again in an hour: the pipe, the attached target,
`Page.addScriptToEvaluateOnNewDocument` for the knobs, a synthetic key, a
`requestAnimationFrame` loop over `getImageData`, and `Page.captureScreenshot`
with a `clip` and a `scale` for a magnified look, which is how the hit-box
overlay and the metrics panel got theirs.

## `Settings` — the point of the project

Eight groups, alphabetical, every field a number or a bool so one menu can edit
all of them. **40 knobs across 11 nesting levels**, because `Feel` holds four
sub-structs of its own.

```go
type Settings struct {
	Animation AnimationSettings // CullOffscreen, FrameCount, FrameDuration
	Audio     AudioSettings     // MusicVolume, PitchJitter, SfxVolume, Voices, Volume
	Camera    CameraSettings    // DeadzoneHeight/Width, Lookahead, Smoothing, Snap
	Debug     DebugSettings     // ShowHitBoxes, ShowMetrics
	Feel      FeelSettings      // Heavy, Light, Medium, Shake
	Render    RenderSettings    // Background, Interpolate, PixelSnap, Smoothing
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
would give GPU frame time is switched off in Chrome for almost everyone. That
row reads `n/a — no browser API`, and `TestMetricsTextFits` pins it to the
width of the row it shares with its label.

| Row | Where it comes from |
|---|---|
| fps, frame p50 / p99 | `performance.now()` deltas into a 120-slot ring; percentiles sort a preallocated scratch array, so the overlay allocates nothing |
| update / draw ms | `now()` at the phase boundaries. The overlays are timed separately and excluded, so the overlay never hides inside the number it reports |
| main thread | work ms over the budget, where the budget is the **measured** median frame interval, not a hardcoded 16.7 — a 120 Hz display has half of that |
| entities / drawn | drawn is after the camera threw the off-screen ones away |
| go heap | `runtime.ReadMemStats`, sampled **once a second**: under TinyGo's conservative collector this walks the heap and would show up in the frame time it is meant to explain |
| wasm mem | the loader publishes `instance.exports.memory`; Go reads `.buffer.byteLength` fresh each time, because growing the memory detaches the old buffer |
| js heap | `performance.memory.usedJSHeapSize` — Chromium only, 0 elsewhere. In `Stats`; the panel does not draw it |
| gpu | `WEBGL_debug_renderer_info` on a throwaway context that is then released; prints `(masked)` when the browser masks it |
| long frames | `PerformanceObserver` on `long-animation-frame`, falling back to `longtask` — Chromium only, `-1` elsewhere. In `Stats`; the panel does not draw it |
| cpu / gpu load | `n/a — no browser API`, one row for both because the answer is the same |

Below the rows, a 60-bar frame-time graph: amber past the budget, red past twice
it. One fill per bar — merging runs of one colour would be fewer calls into the
browser, but every bar has its own height, and the heights are the reason to draw
a graph.

`TestMetricsAllocFree` asserts `AllocsPerRun` is 0 for adding a frame and taking
a percentile. It is the one test that does not run in parallel, because
`AllocsPerRun` counts the whole process.

**The heap row is real, and this is now checked rather than assumed.**
`targets/wasm.json` sets `"gc": "precise"`, and `src/runtime/gc_blocks.go` —
built for `gc.conservative || gc.precise` — assigns `HeapAlloc`, along with
`Alloc`, `HeapInuse`, `HeapObjects`, `Mallocs`, `Frees`, `TotalAlloc` and
`NumGC`. `metrics.go` reads `HeapAlloc`, so the row shows a number TinyGo
actually fills and the "delete it rather than ship a constant" branch does not
apply. The collector on this target is the *precise* one and not the
conservative one this file and `metrics.go` both used to name; the reason for
sampling once a second survives the correction, because `ReadMemStats` walks
the block metadata either way.

**Two lines did not fit, found on the first magnified look, 2026-09-10.** A
row's label is drawn from the panel's left edge and its value from its right,
and neither is clipped to the panel, so a pair wider than the row is not cut
short — the value is painted over the label. `cpu / gpu load` read
`cpu / gpu n/ad— not exposed by browsers`: the value was 29 columns and the
label 14, on a row that has 38 by the menu's own arithmetic — 268 px less
twice the 6 px padding, at Menlo's 0.6023em advance for 11 px, which is 6.63
px a column. And the GPU line ran off the panel onto the scene, because Chrome
reports the card as `ANGLE (Apple, ANGLE Metal Renderer: Apple M4 Pro,
Unspecified Version)`, 70 columns for the 12 that name it. The value is
`n/a — no browser API` now, 20 columns; `gpuName` drops ANGLE's vendor, its
backend's version and the Metal backend's own prefix, keeps a browser's
`(masked)` through any cut, and `clip` cuts what is left to the row with an
ellipsis, counting runes rather than bytes the way the menu's hint learned
to. `hudCols` states the budget with its arithmetic, `metricsRows` lays the
rows out apart from the draw so a test can hold them, and
`TestMetricsTextFits` holds every row at its widest — 999.9 ms everywhere,
99,999 entities, a heap of 1023.9 K — to the budget with a space between
label and value, and the GPU line under a 200-column name; `TestGPUName`
pins Chrome's spelling on macOS, Windows and Linux. It cost 614 bytes, 395
of them the unwrapping. Written with `strings.CutPrefix`, `TrimSuffix` and
`Cut` it cost 2,011, and with `strings.LastIndex` and
`utf8.RuneCountInString` as well 2,218: each brings code a TinyGo build pays
for, so the prefix is compared by hand and the two searches are loops of
their own. The `js heap` and `long frames` rows in the table above are in
`Stats` and not on the panel, which draws eight rows and the graph; the
panel with the fix in is the look in a browser that step 4 of *Verification*
asks for, in Chrome.

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

## Aseprite sheets — milestone 5, done

Every animation used to be exactly 8 frames of exactly 100 ms in its own row.
Aseprite's own export carries named tags, real frame counts and a duration per
frame, and now the engine reads it.

```sh
aseprite -b assets/lab.aseprite \
  --sheet web/static/img/lab.png --sheet-type rows --sheet-width 256 \
  --data web/static/img/lab.json --format json-array --list-tags
```

`ParseSheet` turns that into a `Sheet` of `Frame`s (rect plus duration) and
`Tag`s (name, from, to, direction), a game says `e.Play(i, "walk")`, and
`GridSheet(cols, rows, w, h, ms)` builds the old convention as the same
`Sheet`. One code path, two ways in.

**Aseprite was installed all along.** This file said it was not, and the design
above it planned around that: ship a placeholder fixture and have the test
`t.Skip`. It is at `/Applications/Aseprite.app/Contents/MacOS/aseprite` — a
macOS bundle, so it is not on `PATH` and `which aseprite` says nothing, which
is presumably how the claim got written down. The fixture is a real export.

### `make sheets` was overwriting the art with something unreadable

The recipe passed `--sheet-type packed --shape-padding 1`. The committed
`lab.png` is a 256x384 grid — 8 columns of 12 rows — and the packed export is
3072x32, one long strip. Running the project's own documented command replaced
the art with a layout the engine could not read, and nothing noticed, because
nothing runs `make sheets` in a gate and the result is a PNG.

This file documented a third variant again, `--sheet-type rows` with no width,
which is the same 3072x32 strip.

The fix is `--sheet-type rows --sheet-width 256`, and the width is derived in
the Makefile from `SHEET_COLUMNS` and `SHEET_TILE` rather than typed, so the
grid the exporter is told about is the grid the engine believes in.
`--sheet-columns 8` would say it directly and is what the Aseprite manual
documents; this build ignores it, which the comment records so nobody tries it
again.

**The two assets are different kinds of thing and now get a line each.**
`lab.aseprite` is 96 frames of animation. `tiles.aseprite` is a single 96x160
frame that a tilemap indexes with its own rows and columns — no animation at
all, and a row width would only pad it out to a grid it is not. The old loop
treated them the same.

Both PNGs were regenerated and **decode to identical pixels**, which is the
check that says the pipeline is fixed rather than merely different. The files
differ in their PNG encoding only.

### One code path, two ways in

`GridSheet` is not a compatibility shim bolted to the side. It builds a real
`Sheet` whose frames are the grid's, and `TestGridSheetMatchesTheExport`
asserts that sheet is the committed export, frame for frame and range for
range — the same rectangles, the same durations, the same twelve animations in
the same order. Only the names differ, because a grid says nothing about what
a row is for. That test is the backwards-compatibility proof the design asked
for, and it is what lets `cmd/lab` keep its `rowIdleRight`-style constants: a
row index and a tag index are the same number.

Three things fall out of that:

| | |
|---|---|
| `ImageRow` | stopped meaning only a row. It is which animation is playing: a row on the grid, a tag's index with a sheet. `RowForState` and `Play` write the same field |
| No new per-entity array | a sheet costs the entity store nothing, so the SoA layout and the wasm size are untouched by how a sheet is described |
| `srcRect` | is the one place the two descriptions meet. The draw asks for a rectangle instead of computing one, so there is no second renderer to keep in step |

**Reverse and ping-pong need no state to remember.** `FrameOffset` counts
steps taken, always upwards, and the direction decides which frame a step lands
on. A ping-pong of eight frames is fourteen steps rather than sixteen, because
neither end is shown twice — which also makes a one-shot ping-pong stop after
the way back instead of at the far end. `TestFrameAtStaysInTheTag` walks a
hundred offsets of all three directions and asserts none of them ever addresses
a frame belonging to another tag.

**Two tests carry the lab's switch to a sheet.** It now attaches one, so the
question is whether its sprites moved. `TestGridSheetDrawsWhatNoSheetDraws`
walks every frame of every row through both paths and compares the rectangles;
`TestGridSheetTimesFramesLikeNoSheet` steps two engines on a part-frame time
step and compares which frame each is on. Together with the equivalence against
the export, that is the whole chain: the export is the grid, the grid is a
`Sheet`, and a `Sheet` draws what no sheet drew.

### The scanner

Hand-written, as decided, and the reason has not changed: TinyGo names
`encoding/json` as partially supported because it leans on reflection, the
reflective path is reported at around 119 KB against 27 KB without one, and the
failure mode is a `reflect` panic in a browser on a build every gate called
green.

585 lines including its doc comments. It understands `frames[].frame.{x,y,w,h}`,
`frames[].duration`, `meta.size.{w,h}` and
`meta.frameTags[].{name,from,to,direction}`, and steps over every other value
without interpreting it — which is what makes it survive Aseprite adding
fields, and is asserted rather than hoped for.

Every limit the design named is checked: input at most 4 MiB, depth at most 64,
frames at most 65535, tags at most 1024, rects inside `meta.size` with positive
width and height, `0 <= from <= to < len(frames)`, and `NaN` and infinities
refused — JSON cannot spell them, but `1e400` parses to one, so the check is on
the value rather than on the text. Durations are **clamped** to `[1, 60000]`
rather than refused: Aseprite writes 0 for a frame the artist set to zero, and
that is a sheet worth drawing, not a file worth rejecting.

Five sentinel errors, so a caller can tell "this is not the file I meant" from
"this file is too big to be one": `ErrSheetTooLarge`, `ErrSheetSyntax`,
`ErrSheetTooDeep`, `ErrSheetTooMany`, `ErrSheetRange`.

**The fuzz target asserts what the draw trusts.** Rejecting rubbish is the
easy half; the question that matters is whether anything the parser *accepts*
is safe to draw, because the draw loop rechecks none of it. So the target
re-asserts every invariant on success — rectangles inside the image, tags
inside the frames, durations inside the clamp, names valid UTF-8 — and that
every error is one of the five. 45 seconds, 10.9 million executions, nothing
found.

Two details the fuzzer's shape made obvious. The depth limit is only reachable
through a value being *skipped*: the frames array refuses a `[` where it wants
a frame object long before nesting could run out, so the first draft of that
test was passing for the wrong reason. And a tag index arrives as a JSON
number, so it is refused unless it survives the round trip through `float64`
— otherwise `"from": 0.5` truncates into a valid-looking frame.

**One real bug, found by writing the test rather than by the fuzzer.** A
surrogate pair is spelled as two `\u` escapes, and the reader stepped two bytes
past the backslash instead of one — landing after the `u` rather than on it, so
the second `hex4` read three hex digits and whatever followed. Every tag name
outside the basic plane failed to parse. `TestParseSheetUndoesEscapes` covers
it, and the two attempts before it are worth recording: a heredoc turned the
`\uXXXX` I wrote into the character it denotes, so the first two versions of
the test exercised raw UTF-8 and passed against broken code. The escapes in
that test are now built from `chr(92)` rather than typed.

### What the lab does with it

`cmd/lab` attaches a sheet and spawns with `e.Play(i, "run-right")`, so the
sheet path is what the measured scene actually runs rather than something only
the tests reach.

**It has been run in a browser and the scene animates.** That is the last of
milestone 5's *done means*, and it is the one no test could reach: every
argument that the switch is safe — the export is the grid, the grid is a
`Sheet`, a `Sheet` draws what no sheet drew — is an argument about `srcRect`,
which only the js build ever calls. The tests make the claim; the browser is
what checks it.

It builds that sheet with `GridSheet` rather than parsing `lab.json`, and the
reason is the size gate. The export is 26,901 bytes. Reaching it at runtime
means either embedding it — a third of the module's remaining headroom to
describe a sheet whose every frame is the same size — or a `syscall/js` fetch
and the asynchrony that comes with it. Neither buys the lab anything, because
its frames really are a grid. A game with a packed or per-frame-timed sheet
parses the export; the engine cannot tell the two apart, and that is the point.

The consequence to be honest about: **`ParseSheet` is dead code in
`lab.wasm`**, dropped by the linker, so the module's 274,431 bytes do not price
the scanner. A game that parses an export pays for it, and the 45,569 bytes of
headroom are where it comes from.

### Still open

**The tileset has no sheet and does not want one.** `tiles.aseprite` exports as
one frame, so its JSON carries no tags; `sheetFor` treats a sheet with no tags
as no sheet, and the tilemap keeps indexing with `TilesetCols` and
`TilesetRows`. `web/static/img/tiles.json` is committed because `make sheets`
writes it, not because anything reads it.

**`B` still needs re-reading, and now for three reasons.** The fixed tick moved
`updateStates` off the frame, the draw now calls `srcRect` per drawn entity
instead of doing the arithmetic inline, and it calls `DrawPos` as well. None of
the three should cost much — the first is a loop that was already skipping the
spawned sprites, the other two a bounds check and some arithmetic on numbers
already in cache — but 34,476 was measured against none of them, and the lab's
bounce has moved from the frame to the tick since as well.

## Commands

The baseline Makefile, plus two rule-3 targets for the recurring commands the
gates cannot run. Targets alphabetical, `.DEFAULT_GOAL = check`, GNU Make 3.81
only.

```
make          every gate against the working tree
make ci       the same gates against the commit
make sheets   export the .aseprite sources to their sheets and JSON
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

Three tiers, plus one thing no tier here can reach. All three tools are on this
machine now, so all three tiers run — and milestone 6 added the fourth: a second
project compiles against the published module and its own gates pass, which is
the only check that the API is usable by somebody who did not write it.

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
- **A fuzz target over the Aseprite export**, because the sheet is a file another
  program wrote. It asserts the invariants the draw trusts and never rechecks,
  not merely that rubbish is refused.
- **The grid and the export describe the same sheet**, frame for frame, against
  the committed export rather than a hand-written copy of one.

The lab server was also run for real: correct CSP, `application/wasm`,
digest-based cache busting stamped into the page and every asset URL, `/static/`
answering 404, traversal refused, and both missing browser artefacts named at
boot with the command to run.

**With TinyGo installed — `make wasm`.** Done: 274,431 bytes, against a 320,000
gate that now actually runs, so the size claim cannot rot. This line has been
wrong twice — it still said 234,296 after two commits had moved the number — so
read it against *Gates*, which is updated with the build rather than by hand.
`tinygo build -target wasm -opt=z -size=full -o /dev/null ./cmd/lab` says which
package spends the bytes, and `strings` over the unstripped build says which
functions survived the linker — that pair is what priced the template's 100 KB.

**In a browser — `make run`, then <http://127.0.0.1:8080/>.**

1. `M` — the menu opens over the running scene. Nudge
   `Feel.Heavy.ShakeMagnitude` with the right arrow, close it, press `3`, and see
   the difference without a rebuild.
2. `C` — paste the clipboard into an editor. It must be a Go literal that
   compiles.
3. Reload. The tuning is still there.
4. `H` — the overlay. `cpu / gpu load` must read `n/a`, not a number. Check
   Safari and Firefox too: what is being verified there is that the GPU name
   prints `(masked)`. Done in Chrome on 2026-09-10, magnified, and it read
   `n/a` — painted over its own label, because the value was 29 columns and
   the row has 38; *The metrics overlay* has the fix and the look after it.
   Safari and Firefox are still to look at.
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
8. **Measured on 2026-09-10; the eye's half is still open.** Walk the player
   across the floor, then turn `Render.Interpolate` off in the menu and walk
   back. The motion has to be visibly smoother with it on — that is the whole
   claim of *The draw blends between two ticks*, and no test can make it.
   Nudge `Time.TickRate` down to 20 to see the difference the way a slow
   display would, and watch a spawned sprite as well as the player: the two
   reach the draw by different routes, one through `Simulate` and one through
   the engine's own move bits. A headless Chrome has done all of that with
   the canvas read back every frame — *The blend, measured* has the numbers:
   an even 2 px a frame with the knob on, 0, 0, 6 with it off, on both routes
   and on the wire too. What is left of this step is a person's look at the
   same walk.

## Migrating `game-jam-template` — milestone 6, done

`internal/engine/` is deleted — 802 lines of engine and 455 of its tests — and
`go.mod` requires `github.com/andygeiss/wisp-engine v0.1.0`. The repository is
public and tagged for it; a private module behind a public template would have
been a template nobody but its author could build. `make check`, `make ci` and
`make wasm` are green on the commit, with no `GOPRIVATE` in the environment,
which is the part that proves an ordinary clone works.

The mechanical half was as predicted. `engine.EntityX[i]` is `e.X[i]`,
`engine.StateEntity*` is `wisp.State*`, `engine.CanvasWidth` is `e.Width` and
every surrounding `float64(...)` went with it, `engine.Run(update)` plus
`select {}` is `e.Run(update)`, and the six `AddEntity` call sites are `Sprite`
literals. The game's own code came out 423 bytes *smaller* than before.

### The six that were not mechanical, and what was decided

| | |
|---|---|
| **`Reset` clears `CamTarget` and `InputTarget`** | The one this plan did not predict. `InitializeEntities` left them alone, so `main` set `CamTarget = 0` once at startup and never again. `enterScene` claims the hero after every reset now; without it the camera stops following on `N`, and no gate anywhere would have said so |
| **The hit box moved** | From 20x20 offset six pixels down and right to 20x20 centred, exactly as *Fixes that landed during the move* describes. Same size, different place. It is the one change that wants the boss fight played before it is trusted |
| **`Q`, `E` and `R` are edges now** | The judgement call this plan said to decide rather than let the rename decide. They were level-triggered behind a cooldown, so holding a key re-fired the instant it ended; `JustPressed` is one press, one action. It is a change a player feels, and swapping `JustPressed` for `Down` puts it back. `handleAction2` keeps its state bit, which was never about the edge — it locks the hero's facing while `E` is held |
| **`AnimationFrameDuration` stayed load-bearing** | The migration did not take on per-frame durations: `Engine.Sheets` is left empty, so the grid and its single duration are what the game still runs, and the melee hit window at frames 4 to 6 still means 400 to 700 ms. What did change is the *last* frame test — `e.Animation.FrameCount-1` rather than a literal 7, because `FrameCount` is a knob the `M` menu can move and a literal would leave the swing never ending |
| **Two menus, one keyboard** | The game binds `P`, the engine binds `M`, and the engine takes the keyboard while its own menu is open — so the game's menu freezes underneath rather than double-stepping. Recorded in the commit so nobody "fixes" it |
| **The game's state bits start at 16** | Confirmed: the engine claims bits 0 to 13 and reserves 14 and 15, so `stateAction1 = 1 << 16` is still clear |

Two things fell out that the plan did not list.

**The floor moved to layer -1**, which is the lab's lesson applied before it
could bite. Nothing in the game stood on layer 0, so the tilemap at 0 was safe
as written — but `wisp.Sprite{}` leaves `Z` at zero, and inside a layer the draw
order is by baseline, so the next entity somebody adds without a `Z` would be
painted over by every tile below its middle. Putting the floor under the default
makes the default safe.

**The six hit-stop and shake literals are three named strengths.**
`e.Impact(e.Feel.Heavy)` announces the boss, `.Medium` is the boss taking a hit
and the hero taking one, `.Light` is a monster dying. Three of the six landed on
numbers `Defaults` already carries; the others moved a little and every one of
them is now a knob rather than a literal:

| Call site | Was | Is |
|---|---|---|
| the boss arrives | 10 px for 1000 ms, no stop | `Feel.Heavy` — 10 px for 400 ms, 150 ms stop |
| the boss takes a hit | 4 px for 150 ms, 70 ms stop | `Feel.Medium` — 4 px for 150 ms, 100 ms stop |
| the hero is hurt | 4 px for 150 or 200 ms, 150 or 100 ms stop | `Feel.Medium`, one answer for both call sites |
| a monster dies | 70 ms stop, no shake | `Feel.Light` — and a kill now looks like something |
| the melee dash | 2.5 px for 100 ms | `Feel.Light`'s shake alone, no stop: the stop belongs on the connect, not the wind-up |

`PlaySound` and `PlayMusic` split the way they had to. The music was
`PlaySound(index, 0.25, true)` every frame; it is `e.PlayMusic(index, 1)` now,
and the 0.25 is `Audio.MusicVolume` — which means the volume is a knob too.

Every loop over the entity arrays takes `e.Slots()` and skips with `e.Live(i)`.
The game deletes nothing, so it has no holes to skip and the checks never fire —
but a template is copied, and `len(e.State)` is the habit the free list broke.

### What it cost, and where the bytes went

**411,182 bytes, up from 309,329** — the same TinyGo 0.42 and `wasm-opt -Oz` on
both, the old tree rebuilt from a worktree rather than trusting the committed
file. Per package, before `wasm-opt`: `internal/engine` 17,447 becomes
`wisp-engine` 58,878, and the standard library it pulls adds `internal/strconv`
+5,746, `strings` +4,288 and `slices` +2,924 for the settings text format.

`strings` over the unstripped module says which half of the engine is actually
in there. `ParseSheet` and `GridSheet` are gone, dropped by the linker the way
they are in the lab. `newMenu`, `drawMetrics`, `GoLiteral` and `UnmarshalText`
are all present: about 100 KB of tuning menu, settings format and metrics
overlay, in a build that ships a game.

That is the trade, and it was taken deliberately: the template now hands you a
game you can press `M` on and tune, which is the whole point of the engine
underneath it. **The follow-up it earns, if the number ever matters:** a
`wisp_release` build tag over `menu.go`, `knobs.go`, `metrics.go` and the
settings text format, so a shipped game strips what only a developer looks at.
It splits the code path in two and every gate would have to run both ways, so it
is a milestone rather than a tidy-up, and it is not started.

The template's README said "a 300 KB WASM binary" and that claim had rotted the
moment the engine moved. It says 400 KB now, and `make wasm` there ends with a
size check against `WASM_MAX_BYTES = 430000` — the same lesson as *Fixes on the
first real run*: a claim nothing checks is a memory.

### It has been played

**The game runs on the module.** That is the template's own *Done means* and the
one thing no gate here could reach: every argument that the migration is safe —
the same state bits, the same rows, the same speed, the same tilemap arithmetic
— is an argument about code only the js build ever runs. The gates make the
claim; the browser is what checked it.

Two things are now judgements rather than unknowns, and both are the player's to
make over time rather than a test's. The hit box moved six pixels, from 20x20
offset down-and-right to 20x20 centred, so the boss fight is a slightly
different fight. And `Q`, `E` and `R` fire on an edge instead of a level, so
holding a key no longer re-fires the instant its cooldown ends. Neither is
hard to put back: the margin is `World.HitBoxMargin` in the tuning menu, and
the keys are one `JustPressed` to `Down` each.

## Networking — milestone 7, done

The decision this file said had to come first has been taken, and it is
narrower than the brief it replaces. **The lab becomes the network client, and
the server that serves the lab also runs the world.** The game state lives on
the server. The client sends what the player is trying to do — a direction, a
skill — and gets the world back; the server owns every position and every
cooldown, and the client owns the camera and everything else a player only
looks at. All of it is built, two tabs have played it, and the wire has
been measured. This section is the plan in the order it was built, with each
decision recorded next to what it decided, each step marked as it landed, and
what each one cost.

### The brief

- **Job:** Two people play the same Wisp game in two browsers and see one
  world, and the server is what decides what happened.
- **Why:** A game built on this engine today is one person at one keyboard.
  Making it two meant writing the netcode inside the game, where it could not
  reach the simulation — and until the fixed tick landed, the simulation was
  not reproducible enough to hand to anybody. The two are the same problem: the
  engine owned the world and gave nobody else a way to agree with it.
- **Guardrails:**
  - Zero third-party dependencies, as everywhere else. That is what picks the
    transport rather than taste — *What the browser allows*, below.
  - `go list -deps .` on the root package keeps showing only the standard
    library. The netcode lives under `internal/`, which the root does not
    import; it goes public the day a second consumer needs it and not before.
  - The client compiles under TinyGo and the module stays inside the
    320,000-byte gate. A probe put a `syscall/js` WebSocket with binary framing
    at 6.6 KB; the client half is measured once with `-size=full`, and 10 KB is
    its ceiling. **Missed, and measured**: the client half is 31,736 bytes
    after `wasm-opt`, and *The client is built* says what each part costs and
    what the probe did not count. The module is 46 KB under the gate. Whether
    the ceiling moves or the client shrinks is the brief's owner's call, and
    this file does not make it. What the pricing recommends, on the record:
    retire the per-half ceiling rather than shrink the client. The probe
    priced the socket alone, and the socket came in at 7.8 KB against its
    6.6, so the ceiling was never a number about a client that decodes ten
    messages, keeps a world and prints what the wire costs; the pieces that
    could go are the ones the lab exists to have, and the two that were
    tried saved a kilobyte between them. The module's 320,000-byte gate is
    the claim `make wasm` checks, and it is 45,569 bytes under it. The
    ruling is still Andy's.
  - The server never compiles under TinyGo and never imports the renderer.
  - The client is not trusted. It sends intent — an axis and a skill press —
    and nothing else. Positions, state bits, cooldowns and who hit whom are the
    server's; the camera, the shake, the animation timing and the blend are the
    client's.
  - The lab's measured path survives: `?solo` runs today's scene, `B` included,
    and it is re-measured before this file quotes a new ceiling.
  - `game-jam-template` is not touched.
  - Deployment is not this milestone. Two browsers on one machine is the whole
    claim; the server that ships is milestone 9, with a brief of its own.
- **Done means:**
  - Two browsers on one machine play one world: each moves its own sprite and
    sees the other's, `Q` removes a bouncer only when the server says the hit
    boxes overlapped, and `?solo` still ramps.
  - `go list -deps .` on the root package still shows only the standard
    library.
  - `make check`, `make ci` and `make wasm` are green, and the module is under
    the gate.
  - The RFC 6455 framing has its own tests: the handshake vector, a masked
    frame, an unmasked one refused, a fragmented message, a close handshake,
    and a frame that claims a length it does not have.
  - `SPEC.md` says this is part of the job — it does, as of this plan — and
    every rule this waives is in the README in the six-field form.

### What the decision settled, and what it dissolved

The brief this replaces left four questions open. One is answered and three no
longer exist, and the same fact does all of it: **the client does not
simulate.**

| Was open | Now |
|---|---|
| Does the engine's job change, or is this a second module? | The job widens; `SPEC.md` carries the second sentence. The netcode is subpackages here, because a client has to reach into the entity store and a second module would need the engine to export that reach anyway. |
| Server-authoritative with client prediction, or deterministic lockstep? | Neither. A thin client draws what the server sent, one server tick behind, blended by the draw that already blends. Prediction is a follow-up with a number behind it — the latency at which the lag becomes visible — and lockstep needs bit-identical floats that two browsers do not promise. |
| What tick rate goes on the wire? | 30. The client renders at its own frame rate and the blend covers the gap, which is what it was built for. |
| `randFloat` is the unseeded global, so two machines cannot agree | They do not have to. Only the server rolls dice; the shake on the client is presentation. `random.go` does not change. |

The guardrail about `Settings` splitting on the wire dissolves the same way.
It said `World.Speed`, `World.HitBoxMargin` and `Time.TickRate` have to come
from the server or the tuning menu becomes a cheat menu. With a client that
simulates nothing, a client that sets `World.Speed` to 1 moves at the speed
the server says. One knob still crosses the wire — `Time.TickRate`, because
the blend has to know how long a server tick is — and the client writes the
server's value over it every frame.

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

The engine's own numbers for scale, as they stood when this was written:
`web/static/lab.wasm` was 242,081 bytes against a 320,000 gate, so 77,919
bytes of headroom, of which the client half was to want about 12%. It took
41%: the module is 274,431 bytes now, 45,569 under the gate. That headroom is
the lab's. A game carries its own code as
well — `game-jam-template` is at 411 KB with no gate of this module's to sit
under — so the 10 KB ceiling is a promise about the engine's half and not
about anybody's finished game.

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

### Where it runs

Three things decide who does what. The server is the only place a rule runs.
The wire carries intent one way and the world the other. The client is the
engine's own draw, pointed at somebody else's world.

| The server, every tick | The wire | The client, every frame |
|---|---|---|
| reads each player's last intent and steers their entity with `Move` | Intent: dx, dy, which skills were pressed | reads the keys and sends an intent when it changes |
| counts cooldowns down and honours a skill only at zero | Snapshot: every actor's position, state and row | applies the next snapshot as a tick, so the draw blends between two of them |
| `Tick` — the bounce in `Simulate`, then the engine's own movement | You: your cooldowns | draws cooldown bars from the copy |
| strike hits by `HasCollision`, spawns, dash expiry | Spawn, Despawn, Event | adds and deletes entities, plays the feel of its own skills |
| | Welcome: the tick rate, the world size, which entity is you | follows you with the camera; the floor is built locally and never crosses the wire |

### Shape

```
wisp-engine/
├── cmd/lab/            the client (js && wasm): the ?solo scene, or the replica, camera and HUD
│   └── socket.go       the syscall/js WebSocket; its callbacks only append to a queue
├── cmd/serve/          the page and the tree, as today, plus GET /ws, the world loop and config.go
├── internal/lab/       what both halves agree on: the skills, the world size, the sheet rows, the bounce
├── internal/replica/   pure Go: server slot to engine index, the ordered queue, the jitter buffer
├── internal/wire/      the messages and their bytes; no syscall/js and no net/http, so it tests on the host
└── internal/ws/        RFC 6455: Accept for the server, Dial for tests and bots; frames, close, ping
```

`cmd/serve` grows into the game server rather than a third `cmd/` appearing
next to it. The page, the headers, the cache contract and their tests already
live there, and a second server for one page would put the socket on a
different origin than the page. Its README description — "a development-only
static file server" — changes with it.

The netcode is `internal/`, not `net/`. The library checklist wants the
exported surface to be the minimum, and every package extracted rather than
invented; today the only consumer is `cmd/lab`. The day `game-jam-template`
wants to be networked is the day these become public packages, and until then
v0.2.0's promise is three small hooks.

### Three hooks in the engine

Each is a few lines, each has a test, and `make wasm` says what each cost.

| Hook | What it does | Why the engine and not the client |
|---|---|---|
| `StateRemote`, bit 14 | `updateStates` skips movement and the idle/move rewrite for an entity carrying it, and still picks its row from the state bits | The client stores the server's bits as they are, `StateRemote` added, so `Camera.Lookahead` reads real move bits. Stripping the move bits instead would leave look-ahead silently dead on every network client — a claim nothing checks |
| `Engine.Move(i, dx, dy)` | steers entity i from an axis: the move bits, the facing, the facing lock. `applyInput` became one call to it | A server has N players and no `Input`. Only the sign of dx and dy counts, so a hand-crafted 100 is a 1 |
| `Input.MoveAxis()` | the old `moveAxis`, exported | The client turns keys into an intent the same way the engine turns them into bits |

Bit 14 is one of the two the engine reserved for itself; 15 stays spare. The
tests: a `StateMoveRight|StateRemote` entity does not move over a tick and
still resolves its row; the camera still leads it; `Move` sets the same bits
the keys set.

**Built, and it cost nothing.** `make wasm` went from 239,469 bytes to
239,433 — 36 bytes *smaller*, because the movement moved out of the loop into
one function, `travel`, and the compiler made more of that than the new branch
cost. Nine tests cover the three: `TestMove` steers one entity by the keys and
one by the axis in the same engine and asserts they end in the same state at
the same place; `TestRemote` holds a state the engine would never write — idle
with a move bit — and asserts a tick leaves it alone. One thing the tests
turned up that was already true: an entity with no bit in `RowMask` and no
move bits is skipped whole, so once it stops it keeps the pose it had. A game
always sets a mask, and the test does too.

### The wire

Version 1, big-endian, one WebSocket binary message per wire message, the
first byte the kind. Hand-written on both sides for the reason the sheet
scanner is: `encoding/json` and its `v2` lean on reflection, which TinyGo pays
for in kilobytes and fails at in a browser. `binary.BigEndian` and
`math.Float32bits` are plain functions; nothing here touches `binary.Read`,
`binary.Write` or `fmt`.

| Direction | Message | Body |
|---|---|---|
| client → server | Hello | version u8 |
| client → server | Intent | seq u16, dx i8, dy i8, skills u8 — bit i is skill i pressed since the last intent |
| client → server | Ping | t u32, the client's own clock |
| server → client | Welcome | version u8, tick rate u8, world width u16, world height u16, you u16 (your server slot), tick u32 |
| server → client | Spawn | slot u16, image u8, column u8, row u8, width u16, height u16, x f32, y f32, z i8, alpha u8, state u64 |
| server → client | Despawn | slot u16 |
| server → client | Snapshot | tick u32, n u16, then n × { slot u16, x f32, y f32, state u64, row u8 } — 19 bytes an actor |
| server → client | You | tick u32, n u8, then n × u16: milliseconds of cooldown left, in skill order |
| server → client | Event | tick u32, slot u16 (who), skill u8, x f32, y f32 |
| server → client | Pong | t u32 echoed, tick u32 |

A snapshot is the whole world every tick, on purpose. At 1000 bouncers that is
about 19 KB a tick and 570 KB a second per client, which is fine on one
machine and is the number the lab exists to print. Sending only what changed,
and only what is near, are follow-ups that get written when a number says so.
The number is in — 19,045 bytes a tick and 558 KB/s per client at 1,000
bouncers, measured; *Verification* at the end of this milestone has the table
— and it does not say so yet.

Tests: every message round-trips; a truncated body is an error and not a
panic; a fuzz target over the decoder, under the rule the sheet fuzzer has —
whatever it accepts has to be safe to apply.

**Built.** `internal/wire` is one file of messages and one of tests, and
nothing in it imports more than `encoding/binary`, `errors` and `math`. One
byte moved on the wire while writing it: `You` carries `n u8` ahead of its
cooldowns, so the decoder is self-describing rather than dividing whatever is
left by two. The decoder checks a Snapshot's count against the bytes before it
allocates for it — a count that lies is `ErrShort`, not an allocation —
refuses NaN and the infinities in every position the way the sheet scanner
does, and the tests assert that every prefix of every one of the ten messages
is `ErrShort` and every message plus one byte is `ErrLong`. `FuzzDecode` ran
for 15 seconds — 16.4 million executions — under the rule that whatever it
accepts re-encodes to the bytes it came from, and found nothing.

### The server

The baseline has no rule about WebSockets, hijacked connections or a library
that ships a server — *Baseline gaps*, at the end of this section — so the
server is built to the rules that do exist, and the rest is written down as
conformance notes.

- **`GET /ws`.** `Origin` has to match `Host`. That is RFC 6455's own check,
  and it stays the handler's job because `http.CrossOriginProtection`, which
  wraps the mux the way the checklist wants, ignores GETs.
  `Sec-WebSocket-Version` has to be 13 or the answer is 426. The accept key is
  base64 of SHA-1 over the client's key and the RFC's GUID — SHA-1 by the
  RFC's choice, not for security — and the RFC's own example is the test
  vector: `dGhlIHNhbXBsZSBub25jZQ==` becomes `s3pPLMBiTxaQ9kYGzzhZRbK+xOo=`.
- **After the hijack.** `http.NewResponseController(w).Hijack()` hands back
  the connection with the deadlines net/http set on it still in place, so the
  first thing the socket does is clear them and set its own: a read deadline
  of twice the ping interval, a write deadline per write. That is how the
  checklist's "read, write and idle timeouts" stays true on a connection the
  server's own timeouts no longer see. Frames are read from the returned
  `bufio.Reader`, because it may already hold the first one.
- **Frames.** A client frame has to be masked, or the server closes with 1002.
  A client frame's payload is capped at 1 KiB, checked before anything is
  allocated, or 1009 — an intent is six bytes, and this is the test the
  brief calls "a frame that claims a length it does not have". Fragments are
  reassembled; a control frame is at most 125 bytes and never fragmented; a
  ping gets a pong; a close gets a close.
- **One goroutine owns the engine**, because `Engine` is not safe for
  concurrent use and was never meant to be. `world.Tick()` is a plain method:
  drain the joins, the leaves and the intents; steer each player with `Move`
  and run its skills, where a press counts only when its cooldown is at zero;
  `e.Tick(step)`, which is the bounce in `Simulate` and then the engine's own
  movement; then the rules that read the result — a strike deletes every
  bouncer whose hit box overlaps the striker's, a dash expires — and finally
  one encoded Snapshot for everybody and a `You` for each. The ticker
  goroutine is a wrapper around that method: it runs once before the loop,
  selects on `ctx.Done()`, and treats `context.Canceled` as a normal stop,
  which are the three boxes the background-work checklist has for it. A
  ticker that falls behind drops ticks — the world runs slow rather than away
  — and the log counts them.
- **A slow client cannot slow the world.** Every connection has a bounded
  outbound queue and the loop sends without blocking; a full queue closes that
  connection.
- **The socket goroutines are work a request started and did not wait for**,
  and `srv.Shutdown` never drains a hijacked connection. So the hub counts
  them, shutdown cancels first and waits after — `hub.Wait()` after
  `Shutdown`, hung on `RegisterOnShutdown` — and every player gets a 1001 on
  the way out.
- **No `errgroup`.** `golang.org/x/sync` is the shape the baseline writes
  background work in, and a dependency in a module whose feature is having
  none. Two goroutines under one signal context and a twenty-line wait do
  what it would.
- **Intent validation.** dx and dy are reduced to their sign; a skill bit the
  game does not know is ignored; more than 120 messages a second is 1008; the
  last intent in a tick wins; a held axis persists until the next intent says
  otherwise. What a client can still do is press faster than a human, which is
  a macro and not a cheat.
- **Config.** `cmd/serve/config.go`, flags with environment defaults, the way
  the config pattern wants it: `-addr` (`ADDR`), `-dir` (`WEB_DIR`), `-tick`
  (`TICK`, 30), `-players` (`PLAYERS`, 8), `-bouncers` (`BOUNCERS`, 1000),
  `-log-level`. Nothing under `internal/` reads the environment. The
  `log.Printf` lines become one `slog.Logger`, text handler, built in `main`
  and handed down. `http.MaxBytesHandler` at 1 MiB wraps the mux; on a server
  that only answers GET it does nothing, and the box is ticked.
- **The server's world is the actors.** Players and bouncers, nothing else.
  The floor is 960 tiles that never move and collide with nothing, so the
  client builds it and it never crosses the wire.
- **Tests, under `-race`, over `net.Pipe`.** Two clients join through
  `ws.Dial`; both get the other's Spawn; one sends an intent and the next
  Snapshot has it moved by `World.Speed` times the step; a strike next to a
  bouncer despawns it for both; a client that stops reading is closed; an
  intent with a dx of 100 moves at 1; shutdown closes both with 1001. They
  call `world.Tick()` themselves, so there is no clock to wait on and nothing
  to sleep for.

**The socket is built, as `internal/ws`.** Five hundred lines including its
doc comments, and four decisions that were not in the bullets above:

- **`Close` sends the frame and closes the socket** without waiting for the
  peer's close in return. The peer has the code, which is what a close frame
  is for; a browser reports the close as clean because a close frame arrived
  before the socket went, and the reader goroutine that would have waited for
  the echo is the very thing shutdown is trying to end. When the *peer* closes,
  `Read` echoes the code, closes the socket and returns a `*CloseError`
  carrying it — the same type whichever side chose the code.
- **`Accept` writes its own refusal** — 400 for a request that is not a
  handshake or a key that is not sixteen bytes, 403 for an `Origin` that is
  not this host or no `Origin` at all, 426 with `Sec-WebSocket-Version: 13`
  for any other version — and returns why, so the handler has nothing left to
  write in either case.
- **`Dial` takes a `net.Conn`** rather than an address, so a test hands it one
  end of a `net.Pipe` and a bot would hand it a dialled TCP connection. It
  sends `Origin: http://host`, which is what a page served by that host would
  send, and it refuses a masked server frame the way the server refuses an
  unmasked client one.
- **A hang-up is `io.ErrUnexpectedEOF`, not a close.** A peer that stops
  mid-frame or goes away without a close frame owes no close frame back, so
  the socket is closed and the read error comes up as it is; a read deadline
  is `ReadTimeout` on the `Conn`, set before every frame, which is how the
  server's "twice the ping interval" is carried out.

`MaxMessage` is checked against the length in the header before the payload
is read or allocated for, fragments counted together; it defaults to 1 MiB
and the server sets 1 KiB. The `closed` flag is the one field the reader
shares with the closers, so it is atomic and `-race` is quiet. The tests
drive a real `http.Server` over a pipe listener — one `net.Pipe` per dial,
handed to `Serve`, so the hijack is net/http's own — and cover the five
handshake refusals, every payload size that changes the header's shape in
both directions, the unmasked frame, fragments with a ping between them,
three close handshakes, two length lies and a frame that delivers fewer bytes
than it promised, ping and pong, nine control-frame rules, the masked server
frame, and both deadlines. `FuzzRead` ran 15 seconds — 3.5 million
executions — under the rule that a read ends in a message under the cap or
one of the three errors a caller is written for, and found nothing.

**The server is built.** `cmd/serve` is five files now — `main.go` is the
wiring and nothing else, `config.go` the flags, `pages.go` the static half
that used to be all of it, `world.go` the tick and `client.go` one socket —
and six things were decided in the writing that the bullets above had not
settled:

- **The tick goroutine never writes to a socket.** A close frame is a write,
  and a write to a client that has stopped reading blocks until its deadline.
  A client's `stop` closes a channel and records the code; the socket's own
  writer does the closing when it wakes — at once when it is idle, within one
  write deadline when it is stuck. That is what makes "a slow client cannot
  slow the world" true rather than said.
- **A batch, not a message, is the unit a client is judged by.** The queue
  holds sixty-four batches: a join's Welcome and every Spawn in the world go
  in as one, a tick's Snapshot and You as one, so a crowd of a thousand does
  not close the newcomer for being slow before it has seen anything.
- **A Pong comes from the reader, not from the tick**, so the RTT the client
  prints is the network's and not the network's plus half a tick. The tick
  counter is atomic for that one read.
- **`-crowd` joined the flags** — how many bouncers the world starts with,
  100 by default under the cap of 1,000 — because *Numbers to write down*
  wants snapshot bytes at 100 and at 1,000 bouncers, and reaching a thousand
  by pressing `E` every three seconds is five minutes of pressing. A crowd
  over the cap is refused at boot, as the pair it is.
- **The request log says 101 for a socket**, when the socket closes, with
  how long it lived: `statusWriter` grew a `Hijack` of its own that records
  the switch, and an `Unwrap` so `http.ResponseController` reaches the writer
  underneath for everything else.
- **`http.MaxBytesHandler` and `http.CrossOriginProtection` wrap the mux**
  although a GET-only server gives them nothing to do. They are there so the
  day it answers anything else they already are, and neither touches the
  handshake.

The tests are the seven the bullets asked for and a few more — `parseConfig`
in a table, three first messages the server refuses, a full world, a hundred
and twenty-one pings in a second — and they run the real routes over
`net.Pipe` inside a `synctest` bubble: "the reader has posted the join" is
`synctest.Wait()`, "the stuck writer has reached its deadline" is
`synctest.Sleep`, and the package takes a second and a half under `-race`
with no clock anywhere in it. `log.Printf` is `slog` now, a text handler
built in `main` and handed down, and the boot line logs the config as one
group.

### The lab's rules — `internal/lab`

What both halves have to agree on, in one package with no `syscall/js` in it:
the skills, the world size, the sheet's rows, and the bounce. The skills are
the template's keys and cooldowns, so the day that game is networked its `Q`,
`E` and `R` map one to one.

| Key | Skill | Cooldown | What the server does | What it proves |
|---|---|---|---|---|
| `Q` | strike | 1000 ms | deletes every bouncer whose hit box overlaps the striker's — `HasCollision`, on the server's positions | collision, despawn and the cooldown gate are the server's |
| `E` | spawn | 3000 ms | adds ten bouncers at the player, up to `-bouncers` | spawn on the wire — and the networked lab's ramp |
| `R` | dash | 5000 ms | `SpeedFactor` 4 for 300 ms, on a server-side timer | a timed effect nobody can extend from a browser |

Cooldowns count down per tick, on the server. A press arrives as a bit in an
intent and is honoured only when the cooldown is at zero; the client may grey
the key from its copy, but the copy decides nothing. The randomness — where a
bouncer starts, how fast it goes — is the unseeded global on the server, which
is the one place it now runs.

Tests: a second press inside the cooldown does nothing; a strike takes only
the bouncers that overlap; a spawn past the cap adds none; a dash ends on its
own tick; the bounce turns around at the world's edge.

**Built, and the solo scene runs on it.** `internal/lab` holds the constants
`cmd/lab` used to keep for itself — the images, the rows, the sheet's shape,
the world and its layers — and three things that are rules. `Player` is one
player's cooldowns and dash: `Fire` honours a press only when that skill's
cooldown is at zero, and `Tick` runs the cooldowns and the dash down and keeps
the player inside the world. `Bouncers` is the crowd, a velocity each: its
`Simulate` is the bounce that used to live in `cmd/lab`, and its `Struck` is
the strike. `Setup`, `BuildFloor` and `ClampToWorld` are the parts of the
scene both halves build. `cmd/lab` lost 113 lines to it and spawns, despawns
and bounces through `Bouncers` now, so the sprites the ramp measures are
moved by the very code the server will move them by. Two small things were
decided while writing it: a spawn at the cap still fires and starts its
cooldown, because the gate is about the press and the cap is about the world;
and a dash sets `SpeedFactor`, so the engine's own movement carries it and
the snapshot never has to. Ten tests, the five above among them. It cost the
module 1,041 bytes, 241,040 to 242,081 — the price of the shared package in a
build that does not yet talk to anybody.

### The client — `cmd/lab` and `internal/replica`

- **Two modes, one module.** `?solo` in the URL is today's scene — the
  spawner, the ramp, `B`, all of it. Without it the lab connects to `/ws` on
  the page's own origin. The size gate covers both.
- **On Welcome** the client sets `Time.TickRate` from the server and writes it
  again every frame — the saved menu may still hold the 20 from step 8's
  experiment, and the blend needs the server's number — sets the world size,
  builds the floor locally, sets `InputTarget` to -1 and `RowMask` to 0, and
  points `CamTarget` at the entity the server said is you.
- **An intent a frame.** `MoveAxis()` for the direction and `JustPressed` for
  `Q`, `E` and `R`, read in the update given to `Run`, because that is where
  edges belong: a frame can carry two ticks or none. It is sent when it
  changes and every 500 ms regardless, and a `Ping` goes once a second for the
  RTT row.
- **The engine's `Simulate` is the applier.** Every message is queued in the
  order it arrived. Each tick, `Simulate` applies messages up to and including
  the next Snapshot: a Spawn is an `Add`, and the slot map learns the index; a
  Despawn is a `Delete`; a Snapshot writes `X`, `Y`, the state with
  `StateRemote` set, and the row — resetting the frame when the row changed,
  the way `Play` does. `Tick` has already copied `PrevX` from `X`, so the
  previous snapshot and this one are exactly the pair `DrawPos` blends, and
  the client renders one server tick behind. That is the whole reason the
  blend was built before this.
- **The jitter buffer** is a depth rule, pure Go, with a test for each line.
  The target depth is one snapshot. An empty queue holds for a tick, which is
  a freeze and not a guess. More than two queued applies two per tick — a
  brief fast-forward rather than a pop. More than 64, which is a tab that
  slept, applies everything at once.
- **A hit stop still freezes the picture**, because it stops the client's
  ticks, and the buffer pays it back at double speed afterwards. `P` is gone
  in network mode: the server's world does not pause. `B`, `]` and `[` are
  solo-only.
- **The HUD** is three cooldown bars from the `You` message, a marker over
  your own sprite at `DrawPos`, and a net line, which is the lab doing its job
  again: `rtt`, `in KB/s`, `snapshot B`, `buffer`, and how often it held or
  fast-forwarded. Your own skills fire `e.Impact(e.Feel.Light)` when their
  Event comes back; other people's do not.
- **`socket.go`** is the one `syscall/js` file outside the engine: a WebSocket
  with `binaryType` set to `arraybuffer`, whose `onmessage` copies the bytes
  out with `js.CopyBytesToGo` and appends them to the queue. No goroutine and
  no channel — the callbacks run on the frame loop's goroutine, the same way
  the key events already do.

**The replica is built.** `internal/replica` is one type: an engine, a map
from the server's slot to the engine's index, a queue of messages in arrival
order, and `Tick`, which a client hands to the engine as its `Simulate`. Two
things were decided while writing it. **A server tick's batch ends with its
Snapshot**, and the server sends everything else the tick has to say — the
`You` included — ahead of it, so "apply up to and including the next
snapshot" applies the whole tick; the `You` used to trail the snapshot and
the cooldown bars would have run a tick behind. And **a Pong is not the
world's**: `Push` drops it, because the client has to time it with its own
clock the moment it arrives, and the replica has no clock. Otherwise it is
the depth rule as written — one snapshot a tick, nothing when there is
nothing (`Held` counts it, but only once a first snapshot has been applied,
because before that it is loading), two when more than two are waiting
(`FastForwards`), all of them past sixty-four (`CatchUps`) — plus three
details the tests pin: a Spawn for a slot the client already knows replaces
the old entity, so a slot the server reuses never answers to two; an actor
in a snapshot nobody spawned is ignored rather than added; and a row that
changed resets the frame the way `Play` does while a row that did not keeps
it, so the animation stays the client's to time. `OnEvent` is a callback
rather than a list, because a frame may run two ticks or none and a list
read once a frame would drop the first tick's events. Twelve tests, one of
them the pair: two snapshots on two ticks, and `DrawPos` half a tick later is
exactly halfway between them.

**The client is built**, and it is four files: `main.go` loads the images,
puts the lab's rules on the engine and picks a mode — `?solo`, matched
exactly, is today's scene in `solo.go`, unchanged but for spawning through
`lab.Bouncers`; anything else is `net.go`, and `socket.go` is the one
`syscall/js` file outside the engine, a WebSocket whose three callbacks do
nothing but hand bytes to Go. Network mode is what *The client* above says:
`InputTarget` -1 and `RowMask` 0, the floor built locally, the replica's
`Tick` as `Simulate`, an intent when the axis or a press changes and every
500 ms regardless, a ping a second, the server's tick rate written over the
knob every frame, the camera on your entity once its Spawn has come, and a
HUD of a marker over you, three cooldown bars from the server's copy, and the
net line — `rtt`, `in KB/s`, `snapshot B`, `buffer`, `held`, `ff`, `catch`
and the server's tick.

**It cost 31,736 bytes, against a ceiling of 10 KB, and the ceiling was a
probe of the socket alone.** `make wasm` went from 242,081 to 273,817. The
pieces were priced by taking them away one at a time and rebuilding, in this
order, after `wasm-opt`:

| Piece | Bytes |
|---|---|
| The floor built locally, the mode switch, an empty update | 1,918 |
| The socket: `dialSocket`, the three callbacks, `js.CopyBytesToGo`, a Hello | 7,753 |
| The decoder, all ten kinds, and a Pong timed on arrival | 8,777 |
| The replica: `Push`, `Tick` and `apply`, and the camera finding you | 6,518 |
| Keys to intents, pings and the byte rate | 2,500 |
| The HUD: the marker, the bars, the net line | 5,155 |
| `?solo` by `strings.Contains` rather than by an exact match | 892, taken back |

The probe's 6.6 KB was the socket, and the socket came in at 7.8. Everything
else is what a client that decodes ten messages, keeps a world and prints
what the wire costs is made of. Three things came out of the pricing:

- **A second type through one interface call cost 11.6 KB.** The first
  measurement of "keys to intents" was 13,268 bytes for forty lines, and
  bisecting them found a bare `n.send(wire.Intent{...})` worth 11,600 on its
  own. `wire.Append(b, m Message)` calls `m.appendTo` through the interface;
  with only a Hello ever passed, TinyGo resolves that call statically, and
  the moment an Intent is passed too it becomes a dispatch over every type
  that could arrive — and whatever that drags in, `wasm-opt` cannot undo.
  Each message now has an exported `Append` of its own, the client encodes
  through those, and `wire.Message`'s doc comment says why. The package
  function `Append` remains for the server, where a kilobyte is nothing.
- **TinyGo's `-size=full` and the module disagree.** The compiler's own
  table put the whole client half at 24 KB before the fix; the module grew
  43. The table is the object sizes before `wasm-opt`, and the module is what
  ships, so the module is the number this file quotes. `-size=full` still
  says which package a byte belongs to, which is what it is for.
- **The boxing in `Decode` is not the problem.** Ten types boxed into
  `Message` cost about 220 bytes each — three fewer kinds saved 666 — and a
  slice in place of the replica's slot map saved 390. Neither was worth its
  churn; both were measured rather than guessed at.

**A real browser has joined the world.** With no Chrome extension to drive,
headless Chrome was pointed at `make run`'s page twice: the server's log shows
the page fetch the module, the images and the sheet, open the socket under
the content security policy as it stands — `ws:` to the page's own host is
`'self'` in Chrome, which is one of the three consoles *Security headers*
asks for — say hello, and join as slot 40 of a world with a crowd of 40. What
a screenshot flag cannot do is hold a page in real time for a few seconds or
press a key, so the frames it caught show the floor and "Click to start" —
and, in `?solo`, the player at the centre of the view — and nothing of the
HUD or the moving world. Two tabs with a person at the keyboard are step 9,
exactly as planned. A day later the same Chrome, driven over its debugging
pipe rather than by a flag, did hold the page, walk the player and show the
HUD — *The blend, measured* says how — and the HUD's own numbers agree with
the bot's table below: a 1,926-byte snapshot with one player and the crowd
of 100, which is the bot's 1,945 less one actor, 56.8 KB/s in, and a round
trip of 5.6 to 6.1 ms from a browser's event loop against the bot's 0.2 from
a goroutine.

### Security headers

The policy does not change. `default-src 'self'` is the fallback for
`connect-src`, and CSP Level 3 counts a `ws:` connection to the page's own
host as `'self'` — a claim the last step checks in three
consoles rather than trusts. Chrome's is checked: the headless run in *The
client is built* opened the socket under the policy as it stands. If a browser disagrees, the change is a row in the baseline's own
record and not a project-local edit, because the security-headers pattern lets
no other document restate the policy. `TestSecureHeaders` keeps pinning the
string either way. The `Origin` check is the server's half: the policy
protects the page, not the server.

### What the README says

Recorded here before the README was written, so the plan and the README
agree; they do, as of step 8. Two waivers, in the six-field form:

- **No `main` package** — the existing waiver's scope widens: `cmd/serve`
  becomes the lab's server — the page, the module and the world — and is still
  never deployed until a milestone says otherwise.
- **Every mutation is a POST route** (`checklists/web-application.md`) — the
  world's mutations arrive over the socket. The rule protects the no-JS
  fallback of a plain form, and a canvas client has no such fallback to
  protect.

And five conformance notes: the hand-written RFC 6455 and the hand-written
wire, for the reason the sheet scanner is hand-written; the deadlines set after
the hijack, which is how the timeout rule is met on a connection the server's
timeouts cannot see; `Shutdown` plus `hub.Wait()`, which is the background-work
pattern's "cancel first, then wait"; `errgroup`'s two jobs done in twenty
lines, because the dependency rule outranks the shape; and `WriteTimeout`
detached by the hijack and replaced, not widened.

`SPEC.md`'s lab line gains `?solo` in the same step, because that is when the
bare URL stops being the solo scene.

### The order it is built in

Each step is green on its own and is its own commit.

0. **Close the two loose ends the client leans on.** Step 8 of *Verification*
   — walk the floor with `Render.Interpolate` on and off and write the verdict
   down, because a network client draws nothing but that blend — **measured,
   on 2026-09-10**: a headless Chrome walked the floor with the canvas read
   back every frame, in `?solo` and on the wire, and *The blend, measured* is
   the verdict; the eye's own look is the one line still open. And
   `Debug.ShowHitBoxes`: a few lines in `pass` against `BoundingBox`, or the
   knob goes — **done**, the few lines, at 1,607 bytes, and looked at the
   same day; *Where it stands* has the shape and what the look found.
1. **The three hooks**, with their tests, and the byte delta from `make wasm`
   — **done**; the delta is 36 bytes down.
2. **`internal/wire`**: the round-trip tests and the fuzz target — **done**.
3. **`internal/ws`**: the handshake vector, masked and unmasked, fragmented,
   close, the length lie, ping and pong, the control-frame rules; a fuzz target
   over the frame reader — **done**.
4. **`internal/lab`**: the skills, the cap, the dash timer, the bounce, and
   their tests — **done**.
5. **`cmd/serve`**: `config.go`, `slog`, the hub, `world.Tick`, `/ws`,
   shutdown, and the `-race` tests over `net.Pipe` — **done**.
6. **`internal/replica`**: the slot map, the ordered queue, hold, fast-forward
   and catch-up, each with a test — **done**.
7. **`cmd/lab`**: network mode, the HUD, the net line, `?solo`; `make wasm`;
   the sizes written into *Gates* — **done**, at 273,817 bytes; the client
   half is 31,736 of them against a ceiling of 10,240, and *The client is
   built* is the accounting.
8. **README, SPEC and Makefile**: the waivers and notes above, the run
   instructions, the key table for network mode, `?solo` in SPEC's lab line;
   the js vet line only if a js-only package appears under `internal/` —
   **done**. No js-only package appeared under `internal/`, so the vet line
   did not change; the README gained the two waivers and the five notes as
   written below, a key table for the wire, one for `?solo`, and the server's
   flags; SPEC's lab line points at `?solo` and says the bare URL is the
   wire; the Makefile's two comments that named the old server and the old
   size say the new ones.
9. **Two browsers**, the list below; the numbers into this file; `make ci`;
   tag `v0.2.0`, with the three hooks named in the tag message — **done**:
   Andy played it in two tabs on 2026-09-09 and it works; the numbers were
   measured the same day and are below; `make ci` is green on the commit that
   writes them down, and `v0.2.0` is the annotated tag on that commit, naming
   `StateRemote`, `Engine.Move` and `Input.MoveAxis`, the blend's `DrawPos`,
   `Place` and `Render.Interpolate`, and what broke.

### Verification

- `make check` and `make ci` green; `go list -deps .` still the standard
  library; `make wasm` under 320,000 and the new size written into *Gates*.
- `go test -race ./cmd/serve`: the piped clients of step 5.
- **Played on 2026-09-09: two tabs, one world, and it works.** Which of the
  lines below were walked one by one is Andy's to say, and Safari and Firefox
  are not claimed. Two tabs on `make run`: both sprites in both tabs; moving one moves it in
  the other; `Q` next to a bouncer removes it in both; `E` adds ten in both;
  `R` is visibly faster and its bar refills over five seconds; a key on
  cooldown does nothing; `World.Speed` nudged in one tab's menu changes
  nothing; `?solo` still ramps; `F` keeps the HUD; Safari and Firefox once,
  for the socket and the console.
- The cheat check: an intent with a dx of 100 moves at 1, and `Q` held down
  moves nothing until the server's cooldown is up.
- **Numbers to write down — written down on 2026-09-09** — because a claim
  nothing measures is a memory. The module's bytes before and after the
  client are in *Gates* and *The client is built*: 242,081 and 273,817. The
  rest were taken with a bot rather than a browser, because a browser cannot
  be asked for sixty round trips or told when to start counting. The bot is
  `net.go`'s loop with nobody at the keys: a host build of the engine
  stepped at 60 frames a second by a `time.Ticker`, the replica's `Tick` as
  its `Simulate`, the floor built locally, an intent every 500 ms, a ping a
  second, the server's tick rate written over the knob every frame, and the
  bytes counted the way the HUD counts them — the payload of every message,
  framing excluded. Two of them dialled `cmd/serve` over TCP on 127.0.0.1
  and held a sixty-second window each, starting at the client's first
  applied snapshot so the join batch is outside it. The server's CPU is
  `ps -o time` before and after the window, divided by the wall clock. Apple
  M4 Pro, Go 1.27.1, the server built the way `make build` builds it. The
  bot was 400 lines under `cmd/bot`, and it was deleted once it had printed
  these, because there is no third `cmd/`.

  | | 100 bouncers, two clients | 1,000 bouncers, two clients | 1,000 bouncers, no client |
  |---|---|---|---|
  | snapshot on the wire | 102 actors, 1,945 B | 1,002 actors, 19,045 B | — |
  | in, per client | 57.3 KB/s | 558.2 KB/s | — |
  | round trip on localhost, 59 pings | median 0.2 ms, at most 0.7 | median 0.3 ms, at most 1.0 | — |
  | held / fast-forwards / catch-ups, a minute at rest | 0 / 0 / 0 | 0 / 0 / 0 | — |
  | server CPU, of one core | 1.1 % | 1.6 % | 0.8 % |
  | server RSS | 18.3 MB | 19.1 MB | 18.8 MB |
  | slow ticks | 0 | 0 | 0 |

  Three things the table does not say on its own. **The snapshot is exactly
  what *The wire* priced**: 7 bytes and 19 an actor, and 19,045 a tick at
  1,000 bouncers is 558 KB/s at 30 ticks, within three per cent of the 570
  that section estimated — a number the lab now prints rather than guesses.
  **The buffer sits where it was designed to sit.** In sixty seconds each
  client applied 1,800 snapshots in 1,800 engine ticks against 1,800 server
  ticks — the two clocks agreed to the tick — and the queue after a frame
  held nothing or one snapshot, never two: no tick found the queue empty
  once the first snapshot was in, and none found three. That is a minute on
  one machine with a quarter-millisecond round trip, so the depth rule has
  been seen to hold and has not yet met a network. **The server is idle.**
  The world alone at 1,000 bouncers is 0.8 per cent of a core; two clients
  at 558 KB/s each add 0.8 more, so a client costs about 0.4 per cent at the
  largest crowd the flag allows. The follow-ups *The wire* names — sending
  only what changed, and only what is near — have their number, and it says
  neither is needed for a crowd of a thousand on a machine like this one.

### Baseline gaps

The baseline is the standing guardrail, and six things this milestone needs
are not in it. They are named here to be handed back, not fixed as a side
effect of this work:

1. No rule about WebSockets — the only stance is an htmx anti-pattern — and
   none about `http.Hijacker`: the deadlines after it, and `Shutdown` not
   draining it.
2. `connect-src` for a same-origin `ws:` — the corpus is silent, so it is
   checked in a browser.
3. "Every mutation is a POST route" has no reading for an API a socket
   carries.
4. `/healthz` without a database, which fires when the server deploys.
5. No project type for a library that also ships a server.
6. The background-work shape is written in `errgroup`, which is a dependency a
   zero-dependency module cannot take.

## PixelLab — milestone 8

The art comes from PixelLab now. Every sprite in the lab was
`game-jam-template`'s four-row sheet and every floor tile was one tile
repeated, because drawing was the slowest step — and the engine's two-way
facing hid that a top-down game faces eight ways. PixelLab's MCP generates a
character with eight rotations in minutes, any animation of it for one
generation a direction, a Wang tileset in two minutes and a prop in one, and
the account has 2,000 generations a month. The analysis of 2026-09-10 read
the docs, the OpenAPI spec and the server's own engine guides, then measured
what the docs left out on one probe tileset and the mage that was already on
the account; Andy took four decisions on it the same day, and this section is
the brief, the decisions, the shape and the record.

### The brief

- **Job:** A solo developer describes a character, a terrain or a prop in
  words, and a few minutes later the lab draws it: eight-way, autotiled, at
  the right size, from a manifest of PixelLab IDs that one make target turns
  into the art the engine serves.
- **Why:** Drawing was the slowest step, so the lab never had art of its own
  and the engine never had to face north. PixelLab removes the drawing, but
  the engine cannot yet turn eight ways, cannot tile a shoreline, and trusts
  a padded cell for its hit box and its draw order — so generated art would
  sort late and collide on air until those three are fixed.
- **Guardrails:**
  - Zero third-party dependencies, still. The art tool is standard library:
    `net/http`, `archive/zip`, `image/png`, `encoding/json`. It never compiles
    for the browser, so it may use all of them. `go list -deps .` on the root
    package stays the standard library.
  - Everything the browser runs still compiles under TinyGo, the module stays
    under the 320,000-byte gate, and every engine change writes its byte
    delta into *Gates*.
  - The engine's exported API grows by `Engine.Facing` and a wider
    `MaskPose`, and nothing else. The autotiler and the direction table live
    in `internal/lab` until a second consumer wants them.
  - Art is downloaded at build time into `web/static/img` and served from
    there. Nothing is fetched from PixelLab while a game runs, and the
    content security policy does not change.
  - `make art` needs no token: the download URLs are keyed by the asset's
    UUID. The secret in `.env` stays what it is, the MCP's.
  - The generation budget for the lab is about fifty; `get_balance` is read
    before each spend, and every spend is written down here with its cost.
  - `game-jam-template` is not touched. At the default facing the engine
    behaves exactly as before, and a test pins that.
  - Aseprite's export stays a supported input to `ParseSheet`. What goes is
    the lab's use of it, not the feature.
- **Done means:**
  - `make art` turns the manifest into the committed art with no token, and
    running it a second time changes nothing.
  - The lab's hero faces eight ways and walks, idles and strikes with
    PixelLab animations; the floor is an autotiled meadow with a lake and a
    dirt path from two chained tilesets; the bouncers and the props are
    PixelLab art.
  - A host test checks the lab's sheet constants against the committed
    layout JSON, and another checks its Wang table against the committed
    tileset metadata — the pattern of the grid-equals-export test.
  - `Engine.Facing` is 2, 4 or 8; at 2 every existing key in a `RowForState`
    map resolves as it did, and a test says so.
  - `make check`, `make ci` and `make wasm` are green, the module is under
    the gate, and the deltas are in *Gates*.
  - Two tabs still play one world, eight-way on both halves.
  - The README documents the pipeline and widens the waiver; `SPEC.md` names
    the art source.

### What the probe measured

Everything below was taken from the account rather than read, on
2026-09-10. The docs describe the formats; they do not draw them.

**The character spritesheet export** — `/mcp/characters/{id}/spritesheet`,
no token — is one uniform-grid PNG and a layout JSON, and it is exactly the
engine's grid convention with names attached. Every cell is the size of the
largest frame; frames are centred and never rescaled; there are no
durations. Row 0 is the eight rotations in the order south, south-east,
east, north-east, north, north-west, west, south-west; every animation and
direction pair is a row of its own, frames left to right, and a row shorter
than the widest is padded with transparent cells. The JSON carries
`cell_size`, `sheet_size`, `columns` and a `rows` list of `type`, `animation`,
`direction` and `frame_count`; `export_version` is 1.0 and the export is two
weeks old, which is why a test pins it.

**The cell is padded, and the engine trusts the rectangle.** The mage's
figure measured against its cell:

| Cell | Opaque union over every cell | Feet above the cell bottom |
|---|---|---|
| 40 x 40 | 20 x 24 | 9 to 11 px |

`BoundingBox` is the cell minus a margin and the painter's sort uses the
cell's bottom edge, so a 48 px hero over 32 px tiles would sort late and
collide on air. Cropping every cell by the same union rectangle keeps the
pivot consistent across frames and makes the rectangle the figure; that is
what `cmd/art` is for.

**The tileset export** — `/mcp/tilesets/{id}/image` and `/metadata`, no
token — is a 4x4 sheet of 16 tiles and a JSON with, per tile, four corners
as `lower` or `upper`, a `bounding_box` on the sheet, and a `pattern_4x4`.
The sheet is not in Wang-index order, and the docs say to trust only the
boxes; the probe's order, row-major, with the index NW*8 + NE*4 + SW*2 + SE:

```
13 10  4 12
 6  8  0  1
11  3  2  5
15 14  9  7
```

Whether that order is the same for every tileset is not known, which is why
the table is derived from the metadata by a test rather than assumed by
the code. A 12x8 test map rendered from that index off the probe sheet was
seamless at every cell, including a one-cell inlet. The probe cost 4
generations, as the tool warned it would, and took about two minutes.

**What PixelLab does not give a top-down engine**: maps. Map Workshop
exports a flat PNG and nothing else, so a level stays a vertex grid in
code. Its projects, sandboxes and deploys are a Phaser-on-Cloudflare
product, and its sidescroller, isometric, portrait and voice tools are for
other genres.

### Decisions

| Question | Answer |
|---|---|
| The source of the art | PixelLab IDs, in `assets/pixellab.txt`; the `.aseprite` sources and `make sheets` go. Andy, 2026-09-10. |
| Where the host tool lives | `cmd/art`, a third `main`; the no-`main` waiver widens to it, scoped to a build-time tool the library never imports. Andy, 2026-09-10. |
| The hero | New, in v3 mode at 48 px, and the generations are spent. Andy, 2026-09-10. |
| Where the facing count lives | `Engine.Facing`, a field next to `RowMask`; 0 or 2 is today's behaviour. Andy, 2026-09-10. |
| The tileset sheet | Committed as served, byte for byte. The Wang table is hand-written in `internal/lab`, and a host test derives it from the committed metadata and compares. |
| The character sheet | Cropped by `cmd/art` to the opaque union over every cell, one rectangle for the whole sheet, and the layout JSON rewritten to describe the cropped file with the crop recorded in it. |
| The sheet's tags | Hand-written from a fixed row order — rotations first, then each animation in the manifest's order, eight directions each — and checked against the committed JSON by a host test. |
| `MaskPose` | Grows to six bits: the four facings, idle and moving. At Facing 2 the vertical bits are never set, so every existing key is unchanged. |
| Facing 4 on a diagonal | Keep the facing if it is one of the two axes pressed; otherwise the horizontal one. |
| The Aseprite export | Stays a supported input. The committed export moves to `testdata/` as the parser's fixture, so the tests keep reading a real one. |
| Maps | A vertex grid in `internal/lab`, built by the client the way the floor is today. The server still has no floor. |
| The bouncers | A quadruped character, walking in eight directions, its row picked from its velocity at every bounce — on the server, so the row crosses the wire the way it already does. |
| Props | `create_map_object`, one generation each, cropped by `cmd/art` like a sheet, placed by the client on the actors' layer. They collide with nothing, so the server never has them. |
| The rows' order | `cmd/art` sorts a sheet's rows — the rotations, then every animation by name, its directions in PixelLab's order — because PixelLab's own order follows its animation IDs. Settled while building; *The record* says why. |
| The export date | Dropped from the layout `cmd/art` writes: it is when the file was downloaded, not what it is, and it was the one thing that made a second run differ. Settled while building. |

### Shape

```
wisp-engine/
├── assets/pixellab.txt      the manifest: kind, ID, name, one asset a line
├── cmd/art/                 the tool: read the manifest, download, crop, write
├── internal/pixellab/       the two JSON shapes, the Wang table, the crop rectangle; host and tests only
├── internal/lab/            the eight rows, the directions, the level, the autotiler, the bouncers' rows
├── testdata/aseprite/       the old export, as the parser's fixture
└── web/static/img/          what make art writes: hero, fox, props, two tilesets
```

### The order it is built in

Each step is green on its own.

0. **The brief and this record** — done.
1. **Facing**: `Engine.Facing`, the wider `MaskPose`, `Move` at 2, 4 and 8;
   the tests; the byte delta — **done**, 168 bytes, 274,431 to 274,599.
2. **`internal/pixellab`**: the layout and the metadata as Go types, the
   Wang table, the crop rectangle; tests on fixtures — **done**, and the
   row order joined it, for the reason *The record* gives.
3. **`cmd/art` and `make art`**: the manifest, the downloads, the crop, the
   files; the waiver widened — **done**.
4. **The generation**: the hero and its three animations, the fox and its
   walk, four props, the second tileset chained on the first's grass; every
   ID into the manifest, every cost into this file — **done**, 46
   generations in all; the table is in *The record*.
5. **`internal/lab`**: the directions, the rows, the level, the autotiler,
   the bouncers' rows; tests — **done**.
6. **`cmd/lab`**: the images, the floor, the props — **done**.
7. **The old art out**: `assets/*.aseprite`, `make sheets`, `lab.png`,
   `tiles.png`; the export into `testdata/` — **done**.
8. **README, SPEC, the gates, `make wasm`, two tabs** — the first four
   done and green at 278,777 bytes; the two tabs are Andy's, and a headless
   Chrome has drawn both scenes in their place.

### The record

What each step found that the brief did not know.

**The facing cost 168 bytes**, and no existing test moved: at the default
the vertical bits are never set, so every key a two-row game ever built
resolves as it did, and `TestFacing` holds that beside the four- and
eight-way rules.

**The export's row order is no order.** The first full sheet of the ranger
came back with the strike first, the idle second and the walk last — the
order of the three animations' group IDs, which is the order a database
handed them back in. A regenerated walk would have moved every row under it
and every constant naming a row with it. So `cmd/art` sorts: the rotations
first, then every animation by name, its directions in the order the
rotations row lists them, and the layout JSON is rewritten to say so. That
is why each sheet has its own animation indices in `internal/lab` — the
fox's walk is its first animation and the ranger's its third — and why the
test against the committed JSON is the thing that would catch PixelLab
changing its mind again.

**A cell grows with its widest animation.** The ranger was 48 by 48 with
its rotations, and 68 by 68 once the strike's swing was on the sheet;
cropped, it is 52 by 48. The fox asked for at 32 came back on a 48 canvas,
because standard mode widens the canvas around the figure, and cropped to
44 by 34. Both are in `art.go` and both are checked against the files.

**The export date was the one thing that made a second run differ.** Every
layout download is stamped with the moment it was made, and the tool kept
it as it keeps everything PixelLab wrote, so two runs disagreed about one
line. It is dropped now, the stand-in server in the tool's tests stamps
every download differently, and the test that runs the tool twice is what
holds it. The path tileset's two files also changed once, between a fetch
right after generation and one an hour later, and not again; the one
comparison that mattered — two runs, `git status` clean after the second —
holds, and is in *Verification*.

**The generation**, all on 2026-09-10, on the account's Tier 1 plan:

| Asset | Tool and mode | Generations | Time |
|---|---|---|---|
| the probe tileset, water to grass, 32 px | `create_topdown_tileset`, standard | 4 | about two minutes |
| the ranger, 48 px | `create_character`, v3 | 2 | about four minutes |
| the ranger's walk, eight directions | `animate_character`, template `walking-8-frames` | 8 | about three minutes |
| the ranger's strike, eight directions, eight frames | `animate_character`, v3, from a sentence | 8 | about three minutes |
| the ranger's idle, eight directions, four frames | `animate_character`, template `breathing-idle` | 8 | about three minutes |
| the fox, a quadruped on the dog template | `create_character`, standard | 1 | about four minutes |
| the fox's walk, eight directions | `animate_character`, template `walk-8-frames` | 8 | about three minutes |
| the path tileset, grass to dirt, chained on the meadow's grass | `create_topdown_tileset`, standard | 3 | about two minutes |
| a tree, a boulder, a bush, a stump | `create_map_object` | 4 | under two minutes |
| | | **46** | |

The plan caps eight jobs at once, so an eight-direction animation is one
batch and the three of the ranger's ran one after another; that, not the
generation itself, is what set the pace.

**The look, in a headless Chrome, 2026-09-10.** Driven over its debugging
pipe the way *The blend, measured* was — a click to start, a key held, a
capture of the canvas at twice its size — both scenes drew: the lake with
its shore on every edge and in a one-cell inlet, the path's grass matching
the meadow's, the props standing on the actors' layer, the ranger facing
east while walking east and north-west while walking that way, mid-swing on
`Q` with the sword out, and a hundred foxes walking the way they bounce, on
the wire as in `?solo`, with the HUD's bars cooling. Two things it found.
The player spawns at the world's centre, and the centre was where the lake
had been put, so the ranger stood in water: the lake moved up and left, a
test now holds the centre to be grass, and the stump that the move put in
the water moved with it. And the server stamps its cache-busting digest at
boot, so a module rebuilt under a running server is never fetched by a
browser that has the old one — a note for whoever looks next, not a defect.
The console logged nothing.

**Andy's look, 2026-09-10.** He played it and it works; nothing was asked
to change. The fox and the props were generated without a style reference
to the ranger, which shows a little in the fox's line weight, and the tools
for it — a style image, one `reduce_colors` pass over the set — are a few
generations and a `make art` the day it matters.

**Committed in the order it was built**, one commit a step — the facing,
the export reader, the art tool and the art, the lab, the record — after
the overlay fix that was waiting in the same tree, and tagged `v0.3.0`.

### Verification

- `make check` green on the tree, 2026-09-10; `go list -deps .` the
  standard library; `make wasm` at 278,777 bytes under the gate, and the
  sizes in *Gates*. `make ci` green on the commit tagged `v0.3.0`, before
  it was pushed.
- `make art` twice, and the second run changed nothing: measured by the
  checksum of every file under `web/static/img` before and after.
- A headless Chrome drew both scenes — *The record* says what it saw — and
  Andy played it on 2026-09-10: it works. The ramp's number is still to be
  re-read; *What is left* has it.
- Andy's eye, the same day: it works, and no style pass was asked for.

## What is left

Every open item, with who closes it. Nothing here is a follow-up: the
follow-ups are under *Where it stands* and *What this does not do*, each
gated on a number, and milestone 9 — the server that ships — waits for a
brief of its own. The overlay fix of 2026-09-10 and milestone 8 are
committed and tagged `v0.3.0`, so *Gates* speaks for that commit.

1. **The eye's verdict on the blend.** Walk the floor at `Time.TickRate` 20
   with `Render.Interpolate` on and off and say whether 2 px a frame at 60 Hz
   reads as smooth — the last line of step 8 of *Verification*, after *The
   blend, measured* took the rest. Andy, in a browser.
2. **The tile boxes.** With `Debug.ShowHitBoxes` on, every floor tile is a
   red square and the player's box is one of 254. Either the overlay stays
   honest — every entity, tiles included — or `AddTilemap` marks its tiles
   and `eachHitBox` skips them, which spends bit 15, the last spare, and
   hides the boxes of a game whose tiles are walls. Andy's design call;
   *Where it stands* has the look.
3. **The client half's ceiling.** 31,736 bytes against 10,240, measured and
   priced piece by piece in *The client is built*. Retire the ceiling or
   shrink the client: the brief's guardrail carries the pricing's
   recommendation, and the ruling is Andy's.
4. **Safari and Firefox, once.** The socket under `default-src 'self'`, the
   console, and the GPU name reading `(masked)` — step 4 of *Verification*
   and milestone 7's list. Andy, at those browsers; a headless Chrome cannot
   stand in for either.
5. **The ramp, re-read.** 34,476 was measured against a build that no longer
   exists — the fixed tick, the sheet's `srcRect`, the blend's `DrawPos` and
   the bounce on the tick have each moved the per-frame cost since, and
   milestone 8 changed the sprites it counts: a 44x34 fox in place of a
   32x32 frame, and a floor of about 1,150 tiles in place of 960 — and the
   *sort / draw* row has never been read at the ceiling, which is the number
   that says whether a renderer swap could reach the cost at all. Six minutes
   of `B` in a real window; a headless compositor's frame times are not
   comparable. Andy.
## What this does not do

- **No native build.** Real OS-level CPU, RAM and GPU numbers need a second
  platform layer. A follow-up, and a large one.
- **No WebGL2 renderer.** A follow-up, gated on the number `B` produces.
- **No extrapolation past the last tick.** The draw blends between two ticks
  that have happened, which costs up to one tick of lag. Guessing at the tick
  that has not is what a 60 Hz twitch game would want, and it is wrong every
  time something turns around. See *The draw blends between two ticks*.
- **No prediction, no lockstep, no delta snapshots and no interest
  management.** A tab draws what the server sent, one tick behind and
  blended, and a snapshot is the whole world every tick. Each of those is a
  follow-up with a number in front of it — the latency at which the lag
  shows, the crowd at which the whole world is too many bytes — and the net
  line prints the numbers.
- **No deployment.** Two browsers on one machine is the whole claim. The
  server that ships, with an ops listener and a `/healthz`, is milestone 9
  with a brief of its own.
- **No networked `game-jam-template`.** Its rules run once a frame in the
  update given to `Run`, its action windows are indexed off animation frames,
  and its state is package-level globals with one hero at index 0. All three
  move — into `Simulate`, into tick counts, into per-player state — before that
  game can run on a server, and none of that is this milestone's.
- **No particles, tweens, tint, rotation or squash-and-stretch.** None of them
  are representable — `drawImage` is called with source size equal to destination
  size and there are no scale, rotation or tint arrays. They arrive with the
  WebGL2 renderer, where the shader gives them almost for free.
- **No `devicePixelRatio` handling**, deliberately. A fixed 640x360 backing store
  is what makes the lab's frame times comparable between machines.
- **No release build of the engine.** `game-jam-template` ships the tuning
  menu and the metrics overlay inside its game, which is about 100 KB it never
  shows a player. A build tag would strip them; see *Migrating
  `game-jam-template`*.

## Baseline rules waived, on the record

Five, in the README in the six-field format.

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
5. **Every mutation is a POST route** (`checklists/web-application.md`) — the
   world's mutations arrive over the WebSocket at `GET /ws`, as intents. The
   rule protects the no-JS fallback of a plain form, and a canvas client has
   no such fallback to protect. Scoped to `/ws`; every other route is a GET
   that changes nothing.

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
- **The library checklist's first box is checked.** *A second project actually
  imports this*: `game-jam-template` requires v0.1.0 and its own gates are
  green against it. That is what turned this from an extraction with one
  consumer — the shape the rule exists to catch — into a module.

Milestone 7 added the fifth waiver, widened the first to the server the lab
has now, and put five conformance notes beside them: the hand-written RFC
6455 and wire, the deadlines set after the hijack, a shutdown that cancels
first and waits after, `errgroup`'s two jobs done without it, and
`WriteTimeout` replaced rather than widened. They are written out under *What
the README says* in that milestone's section, and the README carries them.
