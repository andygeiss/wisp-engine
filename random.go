package wisp

import "math/rand/v2"

// randFloat returns a number in [0, 1). It is one function so the engine has
// one source of randomness, and so a build that needs a seeded one has one
// place to change.
func randFloat() float64 { return rand.Float64() }
