//go:build js && wasm

// The lab: one scene that exists to be measured and tuned.
//
// Press M for the tuning menu, then nudge a knob and watch the same hit land
// differently. Without ?solo in the URL the scene is played over the wire:
// the server behind the page runs the world, and this half sends what the
// player is trying to do and draws what comes back. With ?solo it is today's
// scene on this machine alone: press ] until the frame time breaks, and the
// number you stopped at is what this renderer can carry.
package main

import (
	"strconv"

	"github.com/andygeiss/wisp-engine"
	"github.com/andygeiss/wisp-engine/internal/lab"
)

// The HUD's font and its quiet colour, shared by both modes.
const (
	font = "12px system-ui, sans-serif"
	dim  = "rgba(255, 255, 255, 0.55)"
)

func main() {
	e := wisp.New(wisp.Config{})
	e.LoadImages("/static/img/lab.png", "/static/img/tiles.png")

	// The sheet, the pose rows and the world size are the lab's rules, shared
	// with the server. The tileset gets no sheet and so keeps the grid
	// convention, which is what a tilemap wants: it indexes its tiles with
	// its own rows and columns, and none of them is an animation.
	lab.Setup(e)

	// An exact match rather than strings.Contains: the query is ours to spell,
	// and the strings package it would pull in is 900 bytes of module.
	if pageQuery() == "?solo" {
		runSolo(e)
		return
	}
	runNet(e)
}

// itoa and ftoa are here so the lab never reaches for fmt, which costs a
// TinyGo build far more than it looks: strconv is a few hundred bytes, fmt is
// tens of kilobytes of reflection. itoa does not even need strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ftoa spells a number with one decimal, for the HUD.
func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
