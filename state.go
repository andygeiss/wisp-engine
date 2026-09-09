package wisp

import (
	"cmp"
	"math"
	"slices"
)

// The engine's state bits. It reads the ones it knows; a game adds its own
// above bit 15, which leaves two spare for the engine to grow into.
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
)

// MaskMove is the four bits that say an entity is being pushed somewhere.
const MaskMove = StateMoveDown | StateMoveLeft | StateMoveRight | StateMoveUp

// MaskPose is the four bits that make a pose: facing left, facing right, idle
// and moving. A game usually starts its [Engine.RowMask] with it.
const MaskPose = StateFaceLeft | StateFaceRight | StateIdle | StateMove

// maskFacing is the pair the movement keys turn an entity around with.
const maskFacing = StateFaceLeft | StateFaceRight

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
func (e *Engine) advanceAnimation(i int, dt float64) {
	e.FrameTime[i] += dt
	if e.FrameTime[i] >= e.Animation.FrameDuration {
		e.FrameTime[i] = 0
		e.FrameOffset[i]++
	}
	if e.FrameOffset[i] < e.Animation.FrameCount {
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

// applyInput writes the movement keys into entity i's move and facing bits.
// Facing is locked while an action outside the pose bits — an attack, a dash —
// runs, so the sprite does not turn mid-swing.
func (e *Engine) applyInput(i int) {
	s := e.State[i]
	lockFacing := s&e.RowMask&^MaskPose != 0
	dx, dy := e.Input.moveAxis()

	s &^= MaskMove
	if dx < 0 {
		s |= StateMoveLeft
		if !lockFacing {
			s = s&^maskFacing | StateFaceLeft
		}
	}
	if dx > 0 {
		s |= StateMoveRight
		if !lockFacing {
			s = s&^maskFacing | StateFaceRight
		}
	}
	if dy < 0 {
		s |= StateMoveUp
	}
	if dy > 0 {
		s |= StateMoveDown
	}
	e.State[i] = s
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
// skipped, which is most of them.
func (e *Engine) updateStates(dt float64) {
	if e.Live(e.InputTarget) {
		e.applyInput(e.InputTarget)
	}

	for i, s := range e.State {
		if s&e.RowMask == 0 && s&MaskMove == 0 {
			continue
		}

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

		if n := vx*vx + vy*vy; n > 0 {
			// Normalize so a diagonal is not faster than a straight line.
			scale := e.SpeedFactor[i] * e.World.Speed * dt / math.Sqrt(n)
			e.X[i] += vx * scale
			e.Y[i] += vy * scale
			s = s&^StateIdle | StateMove
		} else {
			s = s&^StateMove | StateIdle
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
