package wisp

import (
	"cmp"
	"math"
	"slices"
)

// The engine's state bits. It reads the ones it knows; a game adds its own
// above bit 15, which leaves bit 15 spare for the engine to grow into.
const (
	StateAnimated = uint64(1 << iota)
	StateAnimatedLoop
	StateAutoHide
	StateFaceDown
	StateFaceLeft
	StateFaceRight
	StateFaceUp
	StateIdle
	StateMove
	StateMoveDown
	StateMoveLeft
	StateMoveRight
	StateMoveUp
	StateVisible
	// StateRemote marks an entity that somebody else moves — a server, a
	// replay. The engine keeps its row in step with its state bits but never
	// moves it and never rewrites its pose, so a client can hold the move bits
	// a server sent, for the camera's look-ahead, without the entity walking
	// off on its own. It is bit 14, and it comes last rather than in
	// alphabetical order so that every bit before it keeps the number it had.
	StateRemote
)

// MaskMove is the four bits that say an entity is being pushed somewhere.
const MaskMove = StateMoveDown | StateMoveLeft | StateMoveRight | StateMoveUp

// MaskPose is the six bits that make a pose: the four facings, idle and
// moving. A game usually starts its [Engine.RowMask] with it. At the default
// [Engine.Facing] of 2 the vertical facings are never set, so the keys of a
// two-row sheet's map are the four bits they always were.
const MaskPose = StateFaceDown | StateFaceLeft | StateFaceRight | StateFaceUp | StateIdle | StateMove

// The facing bits by axis: the pair the horizontal axis turns an entity with,
// the pair the vertical one does, and all four.
const (
	maskFaceX = StateFaceLeft | StateFaceRight
	maskFaceY = StateFaceDown | StateFaceUp
	maskFace  = maskFaceX | maskFaceY
)

// advanceAnimations moves every animated entity to its next frame when its
// frame time is up.
//
// It runs in the update pass, not the draw pass, so an entity the camera
// cannot see keeps animating and walks into view mid-stride rather than from
// frame zero. Turn [AnimationSettings] CullOffscreen on to get the cheaper old
// behaviour back.
func (e *Engine) advanceAnimations(dt float64) {
	if dt == 0 {
		return
	}
	var l, t, r, b float64
	if e.Animation.CullOffscreen {
		l, t = e.CamX, e.CamY
		r, b = l+e.Width, t+e.Height
	}
	for i, s := range e.State {
		if s&StateAnimated == 0 {
			continue
		}
		if e.Animation.CullOffscreen && !e.ScreenSpace[i] && !e.overlapsView(i, l, t, r, b) {
			continue
		}
		e.advanceAnimation(i, dt)
	}
}

// advanceAnimation moves entity i to its next frame when its frame time is up.
// A one-shot animation stops after the last frame and, with StateAutoHide,
// hides the entity.
//
// How long a frame lasts and how many of them a cycle takes come from
// [Engine.cycle], which reads the entity's [Sheet] when it has one and
// [AnimationSettings] when it does not. Everything below that is the same
// either way, which is why a sheet needed no second animator.
func (e *Engine) advanceAnimation(i int, dt float64) {
	steps, dur := e.cycle(i)
	e.FrameTime[i] += dt
	if e.FrameTime[i] >= dur {
		e.FrameTime[i] = 0
		e.FrameOffset[i]++
	}
	if e.FrameOffset[i] < steps {
		return
	}
	e.FrameOffset[i] = 0
	if e.State[i]&StateAnimatedLoop != 0 {
		return
	}
	e.State[i] &^= StateAnimated
	if e.State[i]&StateAutoHide != 0 {
		e.State[i] &^= StateVisible
	}
}

// Move steers entity i by an axis. dx and dy are each negative, zero or
// positive, and only the sign counts: a value of 100 pushes exactly as hard as
// a value of 1, so an axis that arrived from somewhere untrusted needs no
// clamping first. It sets the move bits the next [Engine.Tick] moves the
// entity by, and turns the entity to face the way it is going, one of
// [Engine.Facing] ways — unless an action outside the pose bits, an attack or
// a dash, is running, so a sprite does not turn mid-swing.
//
// The engine calls it every tick for [Engine.InputTarget], with the movement
// keys. A server calls it for every player with the axis that player sent, and
// a game calls it to drive anything else the way the keys drive the player.
// The bits hold until the next call, so once a frame is enough.
//
// It panics when i is outside the arrays, which is a programmer error.
func (e *Engine) Move(i int, dx, dy float64) {
	s := e.State[i]
	lockFacing := s&e.RowMask&^MaskPose != 0

	s &^= MaskMove
	if dx < 0 {
		s |= StateMoveLeft
	}
	if dx > 0 {
		s |= StateMoveRight
	}
	if dy < 0 {
		s |= StateMoveUp
	}
	if dy > 0 {
		s |= StateMoveDown
	}
	if !lockFacing {
		s = e.face(s, dx, dy)
	}
	e.State[i] = s
}

// face turns state s the way an axis points, one of [Engine.Facing] ways.
//
// At 2 only the horizontal axis turns the entity, so a sprite walking straight
// up keeps looking the way it last walked sideways — which is what every sheet
// with a left row and a right row was drawn for. At 8 the facing is the
// direction of travel: a diagonal sets a bit from each axis and a straight
// move clears the other axis's bit. At 4 a straight move sets one of the four,
// and a diagonal keeps the facing when it is one of the two axes pressed, so
// a sprite strafes rather than flickers, and takes the horizontal one
// otherwise. A zero axis turns nothing, whatever the count.
func (e *Engine) face(s uint64, dx, dy float64) uint64 {
	var fx, fy uint64
	switch {
	case dx < 0:
		fx = StateFaceLeft
	case dx > 0:
		fx = StateFaceRight
	}
	switch {
	case dy < 0:
		fy = StateFaceUp
	case dy > 0:
		fy = StateFaceDown
	}
	switch e.Facing {
	case 8:
		if fx|fy == 0 {
			return s
		}
		return s&^maskFace | fx | fy
	case 4:
		switch {
		case fx != 0 && fy != 0 && s&(fx|fy) != 0:
			return s
		case fx != 0:
			return s&^maskFace | fx
		case fy != 0:
			return s&^maskFace | fy
		}
		return s
	default:
		if fx == 0 {
			return s
		}
		return s&^maskFaceX | fx
	}
}

// overlapsView reports whether entity i's sprite touches the given rectangle.
func (e *Engine) overlapsView(i int, l, t, r, b float64) bool {
	w, h := e.SpriteWidth[i], e.SpriteHeight[i]
	x, y := e.X[i]-w/2, e.Y[i]-h/2
	return x+w >= l && x <= r && y+h >= t && y <= b
}

// sortDrawOrder orders entities back to front: lower Z first, then lower
// bottom edge (painter's algorithm), then lower index. It runs once per frame.
func (e *Engine) sortDrawOrder() {
	slices.SortStableFunc(e.drawOrder, func(a, b int) int {
		if e.Z[a] != e.Z[b] {
			return cmp.Compare(e.Z[a], e.Z[b])
		}
		ya := e.Y[a] + e.SpriteHeight[a]/2
		yb := e.Y[b] + e.SpriteHeight[b]/2
		if ya != yb {
			return cmp.Compare(ya, yb)
		}
		return cmp.Compare(a, b)
	})
}

// updateStates moves every entity by its move bits and picks its sheet row.
// Entities with no move bits and no bit in RowMask — tiles, HUD sprites — are
// skipped, which is most of them. An entity carrying [StateRemote] gets its
// row and nothing else.
func (e *Engine) updateStates(dt float64) {
	if e.Live(e.InputTarget) {
		dx, dy := e.Input.MoveAxis()
		e.Move(e.InputTarget, dx, dy)
	}

	for i, s := range e.State {
		if s&e.RowMask == 0 && s&MaskMove == 0 {
			continue
		}

		// A remote entity's position and pose are somebody else's answer and
		// arrive in the bits; only the row is this engine's to pick.
		if s&StateRemote == 0 {
			s = e.travel(i, s, dt)
		}

		// An action outside the pose bits picks the row on its own; idle and
		// move are dropped from the key so one action needs one table entry.
		key := s & e.RowMask
		if key&^MaskPose != 0 {
			key &^= StateMove | StateIdle
		}
		if row, ok := e.RowForState[key]; ok && e.ImageRow[i] != row {
			e.ImageRow[i] = row
			e.FrameOffset[i] = 0
			e.FrameTime[i] = 0
		}

		e.State[i] = s
	}
}

// travel moves entity i by its move bits and returns its state with the pose
// brought up to date: moving if it went anywhere, idle if it did not.
func (e *Engine) travel(i int, s uint64, dt float64) uint64 {
	vx, vy := 0.0, 0.0
	if s&StateMoveLeft != 0 {
		vx--
	}
	if s&StateMoveRight != 0 {
		vx++
	}
	if s&StateMoveUp != 0 {
		vy--
	}
	if s&StateMoveDown != 0 {
		vy++
	}

	n := vx*vx + vy*vy
	if n == 0 {
		return s&^StateMove | StateIdle
	}
	// Normalize so a diagonal is not faster than a straight line.
	scale := e.SpeedFactor[i] * e.World.Speed * dt / math.Sqrt(n)
	e.X[i] += vx * scale
	e.Y[i] += vy * scale
	return s&^StateIdle | StateMove
}
