# Spec: Wisp Engine

The project-level brief. Every task brief is a delta against this file.

## Job

A solo developer tunes a 2D pixel-art browser game's feel in the browser, sees
what it costs, and pastes the tuned numbers back into their own source.

## Why

Every number that decides how a game feels — a hit stop, a shake, a follow
delay — used to be a constant in Go. Changing one meant a TinyGo rebuild, so
nobody changed it twice, and no number anywhere said what a change cost.

## Guardrails

- Zero third-party dependencies. That is the feature, not a preference.
- The engine keeps compiling with TinyGo. TinyGo is what makes the module small
  enough to load in a second, and it supports less of the standard library than
  Go does — reflection above all, which is why the sheet parser is written by
  hand. `make wasm` is the check, and it fails above the size budget.
- All overlay UI is drawn on the canvas, never as DOM, so it survives
  fullscreen and needs no exception in the page's content security policy.
- The waived baseline rules and the conformance notes live in the README,
  *Baseline deviations*. Nothing is waived unless it is written there.
- The package doc comment in [doc.go](doc.go) is the long form of *Job*. When it
  and this file disagree, this file wins.
- Still v0. The API may break between minor tags, and the tag message says what
  broke.

## Done means

- The [library checklist](https://github.com/andygeiss/baseline/blob/main/checklists/library.md)
  is walked, and every box is checked or waived on the record in the README.
- `make ci` is green on the commit being pushed.
- The lab runs. `make wasm`, `make run`, then at <http://127.0.0.1:8080/>: `M`
  opens the tuning menu, `]` spawns sprites, and the overlay says how many the
  frame time can carry.
