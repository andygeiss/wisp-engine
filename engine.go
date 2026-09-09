package wisp

// Config is what [New] needs. Every field has a working default, so
// wisp.New(wisp.Config{}) gives a 640x360 engine with the tuning menu on M and
// fullscreen on F.
type Config struct {
	// FullscreenKey toggles fullscreen. Empty means "f". [KeyNone] leaves
	// fullscreen out, so a game may use F for itself.
	FullscreenKey string
	// Height of the drawing surface, in pixels. 0 means 360.
	Height int
	// MenuKey opens and closes the tuning menu. Empty means "m". [KeyNone]
	// leaves the menu out, which is what a shipped game wants: with no menu
	// the engine also stops reading and writing the browser's storage.
	MenuKey string
	// Mount is the CSS selector the canvas is appended to. Empty means
	// "main", falling back to the document body.
	Mount string
	// Width of the drawing surface, in pixels. 0 means 640. 640x360 scales to
	// 1080p and 4K by whole numbers, so nothing blurs.
	Width int
}

// KeyNone turns off a key an engine would otherwise claim. Give it to
// [Config.MenuKey] to ship a game with no tuning menu, or to
// [Config.FullscreenKey] to keep F for yourself.
const KeyNone = "-"

// Engine is one game's world: its entities, its camera, its input, and the
// [Settings] that decide how all of it feels.
//
// Entities live in a structure of arrays. Every attribute is its own slice and
// an entity is an index into all of them, so a game reaches into e.X[i]
// instead of calling a getter, and adding an entity allocates nothing once the
// slices have grown.
//
// Settings is embedded, so the knob you nudge while tuning is short —
// e.Feel.Heavy.ShakeMagnitude — and the whole set is still one value the menu
// can save, reset and print.
//
// Use [New]. An Engine is not safe for concurrent use: the browser calls the
// frame loop on one goroutine, which is the only way it is meant to be driven.
type Engine struct {
	Settings

	// The entity store: one slice per attribute, one index per entity. Every
	// slice has the same length, and len(State) is the entity count.

	// Alpha is the opacity, 0 to 1.
	Alpha []float64
	// FrameOffset is the animation frame showing now.
	FrameOffset []int
	// FrameTime is the milliseconds spent on the current frame.
	FrameTime []float64
	// ImageColumn is the sheet column of the sprite's first frame.
	ImageColumn []int
	// ImageIndex is which loaded image, in [Engine.LoadImages] order.
	ImageIndex []int
	// ImageRow is the sheet row. RowForState overwrites it for entities whose
	// state has a bit in RowMask.
	ImageRow []int
	// ScreenSpace draws the entity in canvas pixels, past the camera.
	ScreenSpace []bool
	// SpeedFactor multiplies World.Speed for this entity.
	SpeedFactor []float64
	// SpriteHeight is the drawn height in pixels.
	SpriteHeight []float64
	// SpriteWidth is the drawn width in pixels.
	SpriteWidth []float64
	// State is the entity's state bits. The engine reads the ones it knows; a
	// game adds its own above bit 15.
	State []uint64
	// X is the world x of the sprite's centre.
	X []float64
	// Y is the world y of the sprite's centre.
	Y []float64
	// Z is the draw layer. Lower draws first.
	Z []int

	// RowForState maps a state, masked by RowMask, to the sheet row that draws
	// it. A game fills it once; the engine only looks things up, so it never
	// has to know what an "attack" is.
	RowForState map[uint64]int
	// RowMask picks the bits RowForState is keyed by. Start it with [MaskPose]
	// and add the game's own action bits.
	RowMask uint64

	// CamTarget is the entity the camera follows, or -1 for none.
	CamTarget int
	// InputTarget is the entity the movement keys drive, or -1 for none.
	InputTarget int

	// CamX is the world x of the view's left edge, before the shake. Read it
	// for parallax; writing it lasts until the next frame.
	CamX float64
	// CamY is the world y of the view's top edge, before the shake.
	CamY float64
	// CamShakeX is the shake offset used when drawing, in pixels. It does not
	// move the world, so a hit box never moves with a shake.
	CamShakeX float64
	// CamShakeY is the vertical shake offset.
	CamShakeY float64

	// Height of the drawing surface in pixels. Read it; do not write it.
	Height float64
	// Width of the drawing surface in pixels. Read it; do not write it.
	Width float64

	// Input is the keyboard and the mouse. Ask it Down, JustPressed or
	// JustReleased; nothing here ever needs clearing by hand.
	Input Input

	// Paused holds the world still. The scene still draws and the game still
	// gets its update with a dt of 0, so it can show a menu and read the keys
	// that leave it, but nothing moves and no animation advances.
	Paused bool

	// RenderUI draws screen-space content after the entities: a HUD, a score,
	// a menu. Use [Engine.Rect] and [Engine.Text]. The engine's own tuning
	// menu draws after this, so it stays on top.
	RenderUI func()

	camMaxX, camMaxY float64
	camMinX, camMinY float64

	drawOrder []int

	failed       []string
	fullscreenAt float64

	hitStopLeft  float64
	hitStopScale float64

	fullscreenKey string
	menu          menu
	menuKey       string

	shakeAt    float64
	shakeLeft  float64
	shakeMag   float64
	shakeTotal float64

	stopped bool
	update  func(dt float64)

	metrics metrics
	rt      backend
}

// New creates an engine with the default settings and no entities. It touches
// no browser API; the canvas appears when [Engine.Run] starts.
func New(cfg Config) *Engine {
	e := &Engine{
		Settings:      Defaults(),
		CamTarget:     -1,
		menu:          newMenu(),
		Height:        float64(cfg.Height),
		InputTarget:   -1,
		Width:         float64(cfg.Width),
		fullscreenKey: cfg.FullscreenKey,
		menuKey:       cfg.MenuKey,
	}
	if e.Height <= 0 {
		e.Height = 360
	}
	if e.Width <= 0 {
		e.Width = 640
	}
	if e.fullscreenKey == "" {
		e.fullscreenKey = "f"
	}
	if e.menuKey == "" {
		e.menuKey = "m"
	}
	e.SetWorldSize(e.Width, e.Height)
	return e
}

// SetWorldSize sets the area the camera may show. A world smaller than the
// canvas is centred. [New] sets it to the canvas size.
func (e *Engine) SetWorldSize(width, height float64) {
	e.camMinX, e.camMinY = 0, 0
	e.camMaxX, e.camMaxY = width, height
}

// Step advances the world by one frame.
//
// elapsed is the real milliseconds since the last frame. The game's update,
// given to [Engine.Run], is called with the time the world actually moved by,
// which is 0 while the world is paused or held by a hit stop — so a game that
// moves things by dt stops for free, and a game that reads keys still reads
// them.
//
// [Engine.Run] calls Step once per frame. A test calls it directly.
func (e *Engine) Step(elapsed float64) {
	elapsed = min(max(elapsed, 0), e.Time.MaxStep)

	// The world is simulation and runs on scaled time. The shake is
	// presentation and runs on wall time, so a hit stop still feels violent.
	dt := e.advanceClock(elapsed)

	// The menu reads its keys first, so a key it consumes never reaches the
	// game, and so a knob changed this frame takes effect this frame.
	e.menu.step(e, elapsed)

	t0 := e.rt.now()
	if e.update != nil {
		e.update(dt)
	}
	e.updateStates(dt)
	e.updateCamera(dt, elapsed)
	e.advanceAnimations(dt)
	e.Input.endFrame()
	e.metrics.updateMs = e.rt.now() - t0

	e.metrics.add(elapsed)
	e.metrics.sampleHeap(elapsed)
}

// Impact applies one hit's worth of feel: a hit stop and a shake, in the
// strength the game asked for.
//
//	e.Impact(e.Feel.Heavy)
//
// The numbers live in [Settings], so the call site carries none and the menu
// can change how every heavy hit in the game lands.
func (e *Engine) Impact(i ImpactSettings) {
	e.HitStop(i.HitStopDuration, i.HitStopScale)
	e.Shake(i.ShakeMagnitude, i.ShakeDuration)
}

// HitStop slows the world to scale for duration milliseconds. A scale of 0 is
// a freeze; 0.1 is slow motion. The longer of the running stop and this one
// wins, so a second hit during a freeze cannot cut it short. A duration of 0
// or less cancels the stop that is running.
func (e *Engine) HitStop(duration, scale float64) {
	if duration <= 0 {
		e.hitStopLeft, e.hitStopScale = 0, 0
		return
	}
	if duration <= e.hitStopLeft {
		return
	}
	e.hitStopLeft = duration
	e.hitStopScale = scale
}

// Shake throws the camera up to magnitude pixels away for duration
// milliseconds, fading out along [ShakeSettings.Decay]. A stronger shake
// replaces a weaker one; a weaker one is ignored while one is running. A
// duration of 0 or less cancels the shake that is running.
func (e *Engine) Shake(magnitude, duration float64) {
	if duration <= 0 || magnitude <= 0 {
		e.shakeAt, e.shakeLeft, e.shakeMag, e.shakeTotal = 0, 0, 0, 0
		e.CamShakeX, e.CamShakeY = 0, 0
		return
	}
	if e.shakeLeft > 0 && magnitude <= e.shakeMag {
		return
	}
	e.shakeAt = 0
	e.shakeLeft = duration
	e.shakeMag = magnitude
	e.shakeTotal = duration
}

// Shaking reports whether a shake is still running.
func (e *Engine) Shaking() bool { return e.shakeLeft > 0 }

// Stopped reports whether a hit stop is still holding the world.
func (e *Engine) Stopped() bool { return e.hitStopLeft > 0 }

// advanceClock runs down the hit stop and returns the time the world moves by
// this frame. The stop counts down on wall time, so a slow-motion stop does
// not stretch itself.
func (e *Engine) advanceClock(elapsed float64) (dt float64) {
	scale := e.Time.Scale
	if e.hitStopLeft > 0 {
		e.hitStopLeft = max(e.hitStopLeft-elapsed, 0)
		scale *= e.hitStopScale
	}
	if e.Paused {
		scale = 0
	}
	return elapsed * scale
}

// claimsKey reports whether the engine handles this key itself. Only a claimed
// key has its browser default suppressed: a preventDefault on everything
// steals the browser's own shortcuts, which is what the engine used to do.
func (e *Engine) claimsKey(key string) bool {
	k := normalizeKey(key)
	if k == KeyNone {
		// KeyNone is what a game passes to turn a feature off, so the key it
		// happens to spell belongs to the game like any other.
		return false
	}
	if k == e.fullscreenKey || k == e.menuKey {
		return true
	}
	switch k {
	case "ArrowDown", "ArrowLeft", "ArrowRight", "ArrowUp", " ", "Tab":
		// The browser scrolls the page on these, which is never what a game
		// wants while somebody is playing it.
		return true
	}
	return false
}
