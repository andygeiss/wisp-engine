# Wisp Engine

A 2D pixel-art engine for games that TinyGo compiles to WebAssembly. It is for
one person building one game: entities in a structure of arrays, a Canvas2D
renderer, keyboard and mouse, a camera, and no dependencies at all.

**Press M while your game runs and every number that decides how it feels
becomes editable, on the canvas, without a rebuild.** Nudge a knob, feel the
difference, press `C`, and paste the result back into your source as Go.

## Install

```bash
go get github.com/andygeiss/wisp-engine
```

The import path ends in `-engine`; the package is called `wisp`.

## 30 seconds

```go
package main

import "github.com/andygeiss/wisp-engine"

func main() {
	e := wisp.New(wisp.Config{})
	e.LoadImages("/static/img/hero.png")

	hero := e.Add(wisp.Sprite{
		Height: 32,
		State:  wisp.StateAnimated | wisp.StateAnimatedLoop | wisp.StateVisible,
		Width:  32, X: 320, Y: 180,
	})
	e.CamTarget, e.InputTarget = hero, hero

	e.Run(func(dt float64) {
		if e.Input.JustPressed(" ") {
			e.Impact(e.Feel.Heavy) // a hit stop and a shake, in one call
		}
	})
}
```

WASD or the arrow keys move the hero. `M` opens the tuning menu, `F` goes
fullscreen.

## Why a hit is not a number

The engine it grew out of wrote its feel at the call site:

```go
engine.CamShakeMagnitude = 4.0
engine.CamShakeTime = 150
engine.HitStopRemaining = 70
```

Six places did that, with three different sets of numbers, and changing any of
them cost a TinyGo rebuild. Wisp names the strengths instead:

```go
e.Impact(e.Feel.Light)  // a connect
e.Impact(e.Feel.Medium) // a solid hit
e.Impact(e.Feel.Heavy)  // a finisher
```

The numbers behind those three live in [`Settings`](settings.go), which the
menu edits while the game runs.

## Sprite sheets, either way round

Say nothing and you get the grid: every frame is the sprite's own size, an
animation is a row, and `Animation.FrameDuration` times all of them. It costs
nothing to state and it cannot describe anything else.

Hand it an Aseprite export and frames may be any size, last different lengths,
and play forwards, backwards or back and forth:

```sh
make sheets   # writes web/static/img/lab.png and lab.json
```

```go
sheet, err := wisp.ParseSheet(exported)
e.Sheets = []wisp.Sheet{sheet}   // one per image, in LoadImages order
e.Play(hero, "walk")
```

`ParseSheet` is a hand-written scanner, not `encoding/json`: TinyGo supports
reflection only partly, so the reflective path costs tens of kilobytes of a
module under a size gate and fails as a `reflect` panic in a browser on a build
every gate called green. It reads the six fields the engine needs and steps
over the rest, so a newer Aseprite is not a breaking change, and it checks what
it read — every rectangle inside the image, every tag naming frames that exist,
every duration a real number of milliseconds.

`GridSheet` builds the grid convention as the same `Sheet`, so the two are one
code path with two ways in. That equivalence is a test against the committed
export, which is what says a game can move to a sheet without its sprites
moving.

## Run the lab

The lab is the engine's own playground: a scene that exists to be tuned and
measured.

```bash
make wasm   # needs TinyGo and wasm-opt
make run    # http://127.0.0.1:8080/
```

| Key | What it does |
|---|---|
| `]` `[` | spawn and remove 100 sprites — with `Shift`, 1000 |
| `0` | clear them |
| `B` | **ramp**: add sprites until the frame time breaks, then hold the count |
| `1` `2` `3` | a light, medium and heavy impact |
| `M` | the tuning menu · `H` the metrics overlay |
| `P` | pause · `N` reset the scene · `F` fullscreen |

`B` is the answer to *"how many sprites before it drops below 60?"* — it walks
the count up and stops at the first frame the 99th percentile cannot carry.
That number is the evidence for whether this renderer needs replacing.

### Inside the tuning menu

| Key | What it does |
|---|---|
| `↑` `↓` | move · `Enter` opens a group, or flips a switch |
| `←` `→` | one step · `Shift` ten · `Ctrl` a tenth |
| `Backspace` | reset this knob · with `Shift`, everything |
| `C` | copy the whole settings value to the clipboard, as Go |
| `T` | fire a test impact, so you can feel a change without playing |

Two tools the gates do not need:

```bash
brew install tinygo-org/tools/tinygo   # 0.42 or newer, for Go 1.27
brew install binaryen                  # wasm-opt

Built and measured with TinyGo 0.42.0 (LLVM 22.1.4) and binaryen 132. Neither
is in the baseline's `VERSIONS.md`, so those two numbers live here.
```

## Commands

```sh
make          # every gate against the working tree
make ci       # the same gates against the last commit
make sheets   # export assets/*.aseprite to a sheet and its JSON
make wasm     # compile the lab and copy the browser artefacts
make run      # start the lab server
make test     # go test -race -shuffle=on ./...
make build    # release-shaped binaries in bin/
make fmt      # goimports + go fix
make clean    # rm -rf bin/
```

## Baseline deviations

This project follows the [engineering baseline](https://github.com/andygeiss/baseline).
Four of its rules are waived here.

- **No `main` package** ([project-types/library.md](https://github.com/andygeiss/baseline/blob/main/project-types/library.md))
  — waived 2026-09-09 by Andy. A feel engine is judged by feel, so it ships the
  playground that proves it. Scoped to `cmd/lab`, which only builds for
  js/wasm, and `cmd/serve`, a development-only static file server; the library
  imports neither, and `go list -deps .` shows only the standard library.
- **No hand-written JavaScript** ([stack/html.md](https://github.com/andygeiss/baseline/blob/main/stack/html.md))
  — waived 2026-09-09 by Andy. A browser can only start a WebAssembly module
  from a script, and the module's own size and the GPU's name are not reachable
  from Go. Scoped to `web/static/js/`: `wasm_exec.js` is TinyGo's runtime glue,
  copied by `make wasm` and never edited, and `wasm_app.js` is a 119-line
  loader. No build step, no npm; the policy below is updated to match.
- **CSP `script-src` carries `'wasm-unsafe-eval'`** ([patterns/security-headers.md](https://github.com/andygeiss/baseline/blob/main/patterns/security-headers.md))
  — waived 2026-09-09 by Andy. Browsers refuse to compile WebAssembly under
  `default-src 'self'` alone. It allows WASM compilation only; `'unsafe-eval'`
  and `'unsafe-inline'` stay banned. Scoped to the `csp` constant in
  `cmd/serve/main.go`.
- **`make check` has two lines the baseline's does not** ([stack/makefile.md](https://github.com/andygeiss/baseline/blob/main/stack/makefile.md) rule 1)
  — waived 2026-09-09 by Andy. `GOOS=js GOARCH=wasm go vet` on the engine and
  the lab, because the host vet never sees a `js && wasm` package, and a size
  check on the built module. `ci` runs `check`, so both are in both.

Conformance notes, for the reader who checks the boxes:

- **Nothing is embedded, and that is the rule rather than a deviation from it.**
  `project-types/library.md` says a library ships no `main` package, no
  embedded assets and no CLI; `patterns/go-project-layout.md`, which does want
  a binary carrying its own `web/`, says in its own opening line that it is the
  layout for a web application. So `cmd/serve` reading the tree named by `-dir`
  is compliance twice over — and it is also what makes editing the stylesheet
  and reloading the whole loop.
- **The version is a digest of the served tree, not `debug.ReadBuildInfo`.** The
  lab's assets are on disk rather than embedded, so the binary's identity says
  nothing about them. `treeVersion` in `cmd/serve/main.go` hashes what is
  actually served, which is what the baseline's rule — two builds with different
  assets must not share a version string — is really asking for.
- **`context.Context` is not the first parameter of `Engine.Run`**, the only
  call that blocks. The browser owns the frame loop's lifetime and nothing on
  the Go side can cancel it; `Engine.Stop` is the handle the rule is asking for.
- `make sheets` and `make wasm` are rule-3 targets: the recurring commands the
  gates cannot run.
- No `htmx`: the page has no hypermedia interaction, so the script would do
  nothing.
- No ops listener and no pprof. The lab server is a development tool that never
  deploys, and profiling the game means the browser's own tools.
- `devicePixelRatio` is deliberately unhandled — see `DESIGN.md`.
- TinyGo decides which Go the engine can be built with, so the Go pin cannot
  move ahead of it. 0.42 was the first release to accept 1.27.

## License

[MIT](LICENSE).
