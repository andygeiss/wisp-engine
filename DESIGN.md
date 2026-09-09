---
version: alpha
name: Wisp Engine – Lab
description: One page that hosts the engine's playground, rendered by WebAssembly on a canvas.
colors:
  bg: "oklch(99% 0.002 260)"
  text: "oklch(22% 0.02 260)"
  canvas: "oklch(18% 0.01 260)"
  bg-dark: "oklch(18% 0.01 260)"
  text-dark: "oklch(93% 0.01 260)"
typography:
  body:
    fontFamily: "system-ui, sans-serif"
components:
  canvas:
    backgroundColor: "{colors.canvas}"
---

# Design: Wisp Engine – Lab

## Overview

The page is a frame around one canvas. The engine draws everything inside the
canvas; the page only centres it, scales it, and follows the reader's colour
scheme. Surface style: minimal.

## Colors

Two page roles, light and dark, plus one fixed canvas ground. Every value is the
same string as in `web/static/css/app.css`.

| Role | Light | Dark | Contrast |
|---|---|---|---|
| `--color-bg` | `oklch(99% 0.002 260)` | `oklch(18% 0.01 260)` | — |
| `--color-text` | `oklch(22% 0.02 260)` | `oklch(93% 0.01 260)` | 16.6:1 light, 14.9:1 dark |
| `--color-canvas` | `oklch(18% 0.01 260)` | same | white canvas text 15.6:1 |

## Typography

`system-ui, sans-serif` at the browser's default size. The page shows no visible
text; the `<h1>` is for screen readers.

## Layout

`main` is a grid that centres the canvas and pads it by `--space`
(`clamp(1rem, 0.5rem + 2vw, 2rem)`). The canvas is `min(100%, 640px)` wide with
a 16:9 aspect ratio, so it fits a 320 px phone and stays sharp on a desktop.

## Elevation & Depth

None. One flat surface.

## Shapes

Square corners. Pixel art has no radius.

## Components

- **canvas** — the lab. Dark ground, pixelated scaling, fullscreen on `F`.
- **tuning menu** — drawn inside the canvas on `M`, over the running world. A
  panel of `rgba(18, 20, 26, 0.94)` with a `rgba(255, 255, 255, 0.25)` border,
  the selected knob in yellow and the rest in white at 55% alpha.
- **hit-box overlay** — one-pixel outlines in `rgba(255, 96, 96, 0.9)` over
  every entity's hit box, on `Debug.ShowHitBoxes` in the tuning menu. Red
  because it marks where things collide; translucent so the sprite under it
  still reads.
- **loader failure card** — drawn by `wasm_app.js` on its own canvas when the
  module cannot be fetched, because at that moment Go has nothing to draw on.
  Ground `#12141a`, heading `#ff6b6b`, detail `#e6e6e6`.

## Do's and Don'ts

- Do keep the canvas at a whole-number scale where you can; pixel art blurs at
  fractional scales.
- Don't handle `devicePixelRatio`. A fixed 640x360 backing store is what makes
  the lab's frame times comparable between machines, and a doubled store would
  be four times the fill rate and blurrier, not sharper.
- Don't put visible UI on the page. Every overlay is drawn inside the canvas, so
  it is there in fullscreen too.
