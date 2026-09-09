package wisp

// LoadImages starts loading each image. [Engine.Loading] stays true until
// every image and sound has arrived or failed.
func (e *Engine) LoadImages(paths ...string) { e.rt.loadImages(e, paths) }

// LoadSounds starts loading each sound. The index of a sound is its position
// in this call, which is what [Engine.PlaySound] takes.
func (e *Engine) LoadSounds(paths ...string) { e.rt.loadSounds(e, paths) }

// Loading reports whether images and sounds are still arriving. A game shows
// its own loading screen while it is true; the engine draws one too.
func (e *Engine) Loading() bool { return e.rt.loading() }

// Failed returns the asset paths that did not load. It is empty when
// everything arrived. Check it once [Engine.Loading] turns false: a missing
// file used to leave the engine on "Loading..." for ever.
func (e *Engine) Failed() []string { return e.failed }

// PauseMusic stops the music where it is. [Engine.PlayMusic] picks it up from
// there, which is what a pause menu wants.
func (e *Engine) PauseMusic() { e.rt.pauseMusic() }

// PlayMusic starts sound index looping, or does nothing if it is already
// playing, so a game may call it every frame.
func (e *Engine) PlayMusic(index int, volume float64) {
	e.rt.playMusic(index, volume*e.Audio.Volume*e.Audio.MusicVolume)
}

// PlaySound plays sound index once. A re-trigger takes the next voice, so
// hitting three monsters in one frame is three sounds rather than one.
func (e *Engine) PlaySound(index int, volume float64) {
	e.rt.playSound(index, volume*e.Audio.Volume*e.Audio.SfxVolume, e.pitch())
}

// Rect fills a rectangle in screen space. Call it from [Engine.RenderUI]; a
// menu needs a backdrop, and the engine draws no shapes of its own.
func (e *Engine) Rect(x, y, w, h float64, color string) { e.rt.rect(x, y, w, h, color) }

// Run creates the canvas, starts the frame loop, and blocks until
// [Engine.Stop] is called. update runs once per frame with the time the world
// moved by, before the engine moves entities and draws.
//
// It blocks so a game's main is one call: keeping the Go runtime alive used to
// be the caller's job and is the engine's now.
func (e *Engine) Run(update func(dt float64)) {
	e.update = update
	e.stopped = false
	e.menu.load(e)
	e.rt.run(e)
}

// Stop ends the frame loop and releases the callbacks the browser holds. It is
// safe to call more than once, and safe to call from update.
func (e *Engine) Stop() {
	e.stopped = true
	e.rt.stop()
}

// StopMusic stops the music and rewinds it.
func (e *Engine) StopMusic() { e.rt.stopMusic() }

// Text draws text in screen space. Call it from [Engine.RenderUI].
func (e *Engine) Text(x, y float64, text, color, font, align string) {
	e.rt.text(x, y, text, color, font, align)
}

// draw paints one frame: the world under the camera, then the screen-space
// entities, then the game's own overlay.
func (e *Engine) draw() {
	e.rt.begin(e)
	if e.rt.loading() {
		e.rt.text(e.Width/2, e.Height/2, e.loadingText(), "white", "24px system-ui, sans-serif", "center")
		e.rt.end()
		return
	}
	t0 := e.rt.now()
	e.sortDrawOrder()
	// The sort is timed apart from the draw because they answer different
	// questions. The sort costs what the entity count costs and a different
	// renderer would not touch it; the draw is the part a different renderer
	// replaces. Rolled together, the number cannot say which one a slow frame
	// was spent on — which is exactly what the renderer decision asks.
	t1 := e.rt.now()
	e.rt.drawEntities(e)
	if e.RenderUI != nil {
		e.RenderUI()
	}
	// The overlays are timed separately and left out of drawMs, so the
	// overlay never hides inside the number it is reporting.
	now := e.rt.now()
	e.metrics.sortMs = t1 - t0
	e.metrics.drawMs = now - t1
	e.metrics.drawn = e.rt.drawn

	e.drawMetrics()
	e.menu.draw(e)
	e.rt.end()
}

// eachHitBox calls fn with the hit box of every entity the pass for
// screenSpace draws, culled to the view the way the sprites are. It is what
// [DebugSettings.ShowHitBoxes] draws with, and it lives here rather than in
// either backend so the browser and its headless twin agree on which boxes
// there are.
//
// A box is where the rules have the entity — [Engine.BoundingBox] reads e.X[i]
// — and not where the sprite is drawn, which is up to a tick behind with
// [RenderSettings.Interpolate] on. An invisible entity's box is included: an
// unseen collider is exactly what a debug overlay exists to show. A box of
// zero size is skipped, because it never collides.
func (e *Engine) eachHitBox(screenSpace bool, fn func(l, t, w, h float64)) {
	vLeft, vTop := 0.0, 0.0
	if !screenSpace {
		vLeft, vTop = e.CamX-e.CamShakeX, e.CamY-e.CamShakeY
	}
	vRight, vBottom := vLeft+e.Width, vTop+e.Height
	for i := range e.Slots() {
		if !e.Live(i) || e.ScreenSpace[i] != screenSpace {
			continue
		}
		l, t, r, b := e.BoundingBox(i)
		if r <= l || b <= t || r < vLeft || l > vRight || b < vTop || t > vBottom {
			continue
		}
		fn(l, t, r-l, b-t)
	}
}

// loadingText says what is happening, and names the file when something is
// wrong. "Loading..." for ever is what a missing asset used to look like.
func (e *Engine) loadingText() string {
	if len(e.failed) == 0 {
		return "Loading..."
	}
	return "Missing: " + e.failed[0]
}

// pitch returns the playback rate for one sound effect. Varying it a little
// stops the same hit sound turning into a machine gun when it fires twenty
// times in a row.
func (e *Engine) pitch() float64 {
	j := e.Audio.PitchJitter
	if j <= 0 {
		return 1
	}
	return 1 + (randFloat()*2-1)*j
}
