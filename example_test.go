package wisp_test

import (
	"fmt"
	"strings"

	"github.com/andygeiss/wisp-engine"
)

// A game creates one engine, fills it with entities, and hands the frame loop
// over. Run blocks until Stop, so main is one call.
func Example() {
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
			e.Impact(e.Feel.Heavy)
		}
	})
}

// A hit is a named strength, not a number at the call site. Change what Heavy
// means and every finisher in the game changes with it.
func ExampleEngine_Impact() {
	e := wisp.New(wisp.Config{})
	e.Impact(e.Feel.Heavy)

	fmt.Println(e.Stopped(), e.Shaking())
	// Output: true true
}

// Step advances the world by one frame. A hit stop holds it, so a game that
// moves things by dt stops for free.
func ExampleEngine_Step() {
	e := wisp.New(wisp.Config{})
	i := e.Add(wisp.Sprite{
		Height: 32, State: wisp.StateVisible | wisp.StateMoveRight, Width: 32,
	})

	e.Step(16)
	moved := e.X[i]

	e.Impact(e.Feel.Heavy)
	e.Step(16)

	fmt.Println(moved > 0, e.X[i] == moved)
	// Output: true true
}

// GoLiteral is what the tuning menu's copy key puts on the clipboard: the
// whole settings value as Go source, ready to paste into a game.
func ExampleSettings_GoLiteral() {
	s := wisp.Defaults()
	s.Feel.Heavy.ShakeMagnitude = 14

	src := s.GoLiteral()
	for line := range strings.Lines(src) {
		if strings.Contains(line, "ShakeMagnitude") && strings.Contains(line, "14") {
			fmt.Print(strings.TrimSpace(line))
			break
		}
	}
	// Output: ShakeMagnitude:  14,
}
