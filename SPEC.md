# Spec: Wisp Engine

The project-level brief. Every task brief is a delta against this file.

## Job

A solo developer tunes a 2D pixel-art browser game's feel in the browser, sees
what it costs, and pastes the tuned numbers back into their own source. When
the game is for more than one person, the same engine runs on a server that
decides what happened, and the browser says only what the player is trying to
do.

## Why

Every number that decides how a game feels — a hit stop, a shake, a follow
delay — used to be a constant in Go. Changing one meant a TinyGo rebuild, so
nobody changed it twice, and no number anywhere said what a change cost.

## Guardrails

- Zero third-party dependencies. That is the feature, not a preference.
- The root package imports only the standard library, and `go list -deps .` is
  the check. The netcode lives under `internal/`, which the root does not
  import, so a game that does not need it does not pay for it; it goes public
  the day a second consumer needs it.
- The transport is WebSocket, and that is a consequence rather than a choice. A
  browser has no UDP, and WebRTC and WebTransport both need a dependency the
  first rule refuses. WebSocket is TCP, so the engine promises a 15-to-30 Hz
  authoritative game and not a 60 Hz twitch shooter.
- The client is not trusted. It sends what the player is trying to do — a
  direction, a skill — and nothing else. The server owns every position, every
  state bit and every cooldown; the client owns the camera and everything else
  a player only looks at.
- The engine keeps compiling with TinyGo. TinyGo is what makes the module small
  enough to load in a second, and it supports less of the standard library than
  Go does — reflection above all, which is why the sheet parser is written by
  hand. `make wasm` is the check, and it fails above the size budget.
- All overlay UI is drawn on the canvas, never as DOM, so it survives
  fullscreen and needs no exception in the page's content security policy.
- The waived baseline rules and the conformance notes live in the README,
  *Baseline deviations*. Nothing is waived unless it is written there.
- The art is PixelLab's, named by ID in `assets/pixellab.txt`. `make art` is
  the only way art enters the tree, what it writes is committed, and a
  second run changes nothing. Nothing is fetched from PixelLab while a game
  runs.
- The package doc comment in [doc.go](doc.go) is the long form of *Job*. When it
  and this file disagree, this file wins.
- Still v0. The API may break between minor tags, and the tag message says what
  broke.

## Done means

- The [library checklist](https://github.com/andygeiss/baseline/blob/main/checklists/library.md)
  is walked, and every box is checked or waived on the record in the README.
- `make ci` is green on the commit being pushed.
- The lab runs. `make wasm`, `make run`, then at <http://127.0.0.1:8080/?solo>:
  `M` opens the tuning menu, `]` spawns sprites, and the overlay says how many
  the frame time can carry. The same URL without `?solo` is the wire.
- Two browsers on one machine play one world: each moves its own sprite and
  sees the other's, a strike lands only when the server says so, and `?solo`
  still ramps.
- The lab is drawn from generated art: an eight-way hero that walks, idles
  and strikes, a crowd that walks where it bounces, an autotiled floor and
  props, and `make art` reproduces every file of it.
