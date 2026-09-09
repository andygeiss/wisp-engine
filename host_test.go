//go:build !js || !wasm

package wisp

import "testing"

// What the headless backend records is only there off a browser, so the tests
// that read it back carry the same build tag it does. The js vet compiles the
// test files too, and the browser's backend keeps no record.

// The hit-box overlay is drawn by the backend, so which boxes a frame outlines
// is read back from the headless twin's record of them.
func TestShowHitBoxesDrawsWhereTheRulesAre(t *testing.T) {
	t.Parallel()
	e := New(Config{})
	e.SetWorldSize(4000, 4000)
	mover := e.Add(Sprite{Height: 32, State: StateVisible | StateMoveRight, Width: 32, X: 100, Y: 100})
	e.Add(Sprite{Height: 32, State: StateVisible, Width: 32, X: 3000, Y: 3000}) // off screen: culled
	e.Add(Sprite{Height: 32, Width: 32, X: 200, Y: 100})                        // invisible: still a collider
	e.Add(Sprite{Height: 8, State: StateVisible, Width: 8, X: 300, Y: 100})     // the margin eats it: no box
	e.Add(Sprite{Height: 16, ScreenSpace: true, State: StateVisible, Width: 16, X: 20, Y: 20})

	e.draw()
	if n := len(e.rt.boxes); n != 0 {
		t.Fatalf("%d boxes drawn with the knob off, want none", n)
	}

	// One tick and a bit, so the mover's drawn position and its simulated one
	// come apart: the box has to follow the second.
	e.Debug.ShowHitBoxes = true
	e.Step(20)
	e.draw()

	if n := len(e.rt.boxes); n != 3 {
		t.Fatalf("%d boxes drawn, want 3: the mover, the invisible one and the HUD one", n)
	}
	l, tp, r, b := e.BoundingBox(mover)
	if want := [4]float64{l, tp, r - l, b - tp}; e.rt.boxes[0] != want {
		t.Errorf("the mover's box = %v, want %v, its BoundingBox", e.rt.boxes[0], want)
	}
	if dx, _ := e.DrawPos(mover); dx == e.X[mover] {
		t.Fatal("the mover is drawn where it is simulated, so this frame cannot tell the two apart")
	}
	if e.rt.boxes[0][0] != e.X[mover]-e.SpriteWidth[mover]/2+e.World.HitBoxMargin {
		t.Errorf("the box's left edge is %v, want it at the simulated position %v, not the drawn one",
			e.rt.boxes[0][0], e.X[mover])
	}
}
