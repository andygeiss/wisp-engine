package wisp

import "math"

// updateCamera follows the target, keeps the view inside the world, and runs
// the shake.
//
// It takes two clocks on purpose. The follow runs on dt, the world's own
// scaled time, so slow motion slows the camera with everything else. The shake
// runs on elapsed, real wall time, so a hit stop that freezes the world still
// rattles the screen — which is the whole point of freezing it.
func (e *Engine) updateCamera(dt, elapsed float64) {
	e.follow(dt)
	e.clampToWorld()
	if e.Feel.Shake.IgnoresHitStop {
		e.updateShake(elapsed)
		return
	}
	e.updateShake(dt)
}

// clampToWorld keeps the view inside the world. A world smaller than the
// canvas is centred.
func (e *Engine) clampToWorld() {
	worldW := e.camMaxX - e.camMinX
	worldH := e.camMaxY - e.camMinY
	if worldW <= e.Width {
		e.CamX = e.camMinX + (worldW-e.Width)/2
	} else {
		e.CamX = min(max(e.CamX, e.camMinX), e.camMaxX-e.Width)
	}
	if worldH <= e.Height {
		e.CamY = e.camMinY + (worldH-e.Height)/2
	} else {
		e.CamY = min(max(e.CamY, e.camMinY), e.camMaxY-e.Height)
	}
}

// follow moves the camera toward its target, honouring the dead zone, the
// look-ahead and the smoothing.
func (e *Engine) follow(dt float64) {
	i := e.CamTarget
	if !e.Live(i) {
		return
	}

	// Where the target is drawn, not where the simulation has it: the two are
	// up to a tick apart, and a camera locked to the other one leaves the
	// sprite it is following jittering against a screen that moves in steps.
	tx, ty := e.DrawPos(i)
	wantX := tx - e.Width/2
	wantY := ty - e.Height/2

	// Look-ahead pushes the view the way the entity is moving, so the player
	// sees where they are going rather than where they have been.
	if la := e.Camera.Lookahead; la > 0 {
		s := e.State[i]
		if s&StateMoveLeft != 0 {
			wantX -= la
		}
		if s&StateMoveRight != 0 {
			wantX += la
		}
		if s&StateMoveUp != 0 {
			wantY -= la
		}
		if s&StateMoveDown != 0 {
			wantY += la
		}
	}

	// The dead zone lets the target wander before the camera answers, so small
	// movements do not swim the whole screen.
	wantX = deadzone(e.CamX, wantX, e.Camera.DeadzoneWidth)
	wantY = deadzone(e.CamY, wantY, e.Camera.DeadzoneHeight)

	if e.Camera.Smoothing <= 0 {
		e.CamX, e.CamY = wantX, wantY
		return
	}
	// An exponential catch-up, so the feel does not change with the frame
	// rate: at Smoothing 8 the camera closes 8/e of the gap every second,
	// whether that second is 60 frames or 144.
	k := 1 - math.Exp(-e.Camera.Smoothing*dt/1000)
	e.CamX += (wantX - e.CamX) * k
	e.CamY += (wantY - e.CamY) * k
}

// updateShake runs the shake down and picks the offset it draws with.
func (e *Engine) updateShake(dt float64) {
	if e.shakeLeft <= 0 {
		e.CamShakeX, e.CamShakeY = 0, 0
		return
	}

	e.shakeLeft = max(e.shakeLeft-dt, 0)
	if e.shakeLeft <= 0 {
		e.CamShakeX, e.CamShakeY = 0, 0
		e.shakeMag, e.shakeTotal = 0, 0
		return
	}

	// The offset is re-picked at Frequency, not every frame. A new offset
	// every frame is white noise, which reads as static rather than as a
	// shake, and it looks different on a 60 Hz screen than on a 144 Hz one.
	// shakeAt starts at 0, so the first frame of a shake always picks one.
	e.shakeAt -= dt
	if e.shakeAt > 0 {
		return
	}
	e.shakeAt = 0
	if f := e.Feel.Shake.Frequency; f > 0 {
		e.shakeAt = 1000 / f
	}

	// Decay shapes the fade. 0 holds full strength and cuts at the end, which
	// is what the engine did before the knob existed.
	amp := e.shakeMag
	if d := e.Feel.Shake.Decay; d > 0 && e.shakeTotal > 0 {
		amp *= math.Pow(e.shakeLeft/e.shakeTotal, d)
	}
	e.CamShakeX = (randFloat()*2 - 1) * amp
	e.CamShakeY = (randFloat()*2 - 1) * amp
	if e.Feel.Shake.Snap {
		e.CamShakeX = math.Round(e.CamShakeX)
		e.CamShakeY = math.Round(e.CamShakeY)
	}
}

// deadzone returns where the camera should be when the target may wander size
// pixels before the camera answers.
func deadzone(have, want, size float64) float64 {
	if size <= 0 {
		return want
	}
	half := size / 2
	if want > have+half {
		return want - half
	}
	if want < have-half {
		return want + half
	}
	return have
}
