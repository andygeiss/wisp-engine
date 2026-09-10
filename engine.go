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
// slices have grown. An index keeps meaning the same entity until that entity
// is deleted; see [ID] for a name that outlives one.
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
	// slice has the same length, which is [Engine.Slots] — not the entity
	// count, because a deleted entity keeps its slot for the next one.

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
	// ImageRow is which animation is playing: a row on the grid convention, a
	// tag's index when the image has a [Sheet]. RowForState overwrites it for
	// entities whose state has a bit in RowMask, and [Engine.Play] writes the
	// tag it looked up.
	ImageRow []int
	// PrevX is where the entity was when the tick running now began, and
	// PrevY the same. [Engine.DrawPos] is what reads them, to place a sprite
	// between two ticks; [Engine.Place] is how a game writes them, for an
	// entity that jumps somewhere rather than travelling there.
	PrevX []float64
	PrevY []float64
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

	// Facing is how many ways [Engine.Move] turns an entity: 2, 4 or 8, and 0
	// means 2. At 2 only the horizontal axis turns it, which is what a sheet
	// with a left row and a right row wants, and what every game had until
	// now. At 4 a move sets one of the four facing bits; at 8 a diagonal sets
	// two, one from each axis, so the eight rotations of a top-down sprite
	// key a RowForState map through [MaskPose]. Both halves of a networked
	// game set the same value, because the server is what turns a player.
	Facing int
	// RowForState maps a state, masked by RowMask, to the sheet row that draws
	// it. A game fills it once; the engine only looks things up, so it never
	// has to know what an "attack" is.
	RowForState map[uint64]int
	// RowMask picks the bits RowForState is keyed by. Start it with [MaskPose]
	// and add the game's own action bits.
	RowMask uint64

	// Sheets describes the loaded images: one entry per image, in the order
	// [Engine.LoadImages] took them. A game fills it once, from [ParseSheet]
	// or [GridSheet]. An image with no entry — or a zero Sheet, or one with no
	// tags — draws on the grid convention instead, so a game that never had a
	// sheet never needs one. See [Sheet].
	Sheets []Sheet

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

	// Simulate moves the game's own world, and runs at the fixed rate
	// [TimeSettings.TickRate] sets rather than once per frame. Put movement,
	// physics and anything a rule depends on here: it is called with the same
	// dt every time, so the same input twice gives the same answer twice,
	// which is what a replay — and one day a rollback — needs.
	//
	// Read held keys here with [Input.Down]. An edge belongs in the update
	// given to [Engine.Run], which runs once per frame: a frame can carry two
	// ticks or none, so a JustPressed read here fires twice or not at all.
	Simulate func(dt float64)

	// The entity store's bookkeeping. alive and gen are one entry per slot;
	// free holds the slots a delete left behind, newest first.
	alive   []bool
	free    []int
	gen     []uint32
	live    int
	nextGen uint32

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

	tickAccum float64

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
// The simulation does not run on that time. Step spends it on whole
// [Engine.Tick] calls of a fixed length and carries the remainder, so how far
// anything moves stops depending on how long the frame took. Everything the
// player only looks at — the camera, the shake, the animations, the menu —
// stays on the frame, because it has nothing to agree with anybody about.
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
	e.runTicks(dt)
	e.updateCamera(dt, elapsed)
	e.advanceAnimations(dt)
	e.Input.endFrame()
	e.metrics.updateMs = e.rt.now() - t0

	e.metrics.add(elapsed)
	e.metrics.sampleHeap(elapsed)
}

// Tick advances the simulation by exactly dt milliseconds: the game's
// [Engine.Simulate] first, then the engine's own movement.
//
// [Engine.Step] calls it as often as the frame's time paid for, which is what
// a game wants. Call it directly to run the simulation on a clock that is not
// this machine's — a server's tick, or a replay of one whose input you already
// have — which is why it takes the step instead of reading a clock.
func (e *Engine) Tick(dt float64) {
	// Where everything stands as this tick begins, so the frames drawn before
	// the next one have somewhere to come from. Two slice copies, run whether
	// or not [RenderSettings.Interpolate] is on: the knob belongs to the draw,
	// and a knob switched on mid-scene would otherwise blend from wherever the
	// world was when it was switched off.
	copy(e.PrevX, e.X)
	copy(e.PrevY, e.Y)

	if e.Simulate != nil {
		e.Simulate(dt)
	}
	e.updateStates(dt)
}

// runTicks spends the world time this frame earned on whole ticks and carries
// what is left over into the next frame.
//
// The loop cannot run away. Step has already capped elapsed at
// [TimeSettings.MaxStep], so one frame buys at most MaxStep worth of ticks
// however long the tab was asleep — the same cap that stopped a background tab
// teleporting everything.
func (e *Engine) runTicks(dt float64) {
	step := e.tickStep()
	e.tickAccum += dt
	for e.tickAccum >= step {
		e.Tick(step)
		e.tickAccum -= step
	}
}

// DrawPos returns where entity i is drawn this frame: its own position,
// blended back toward where it stood when the current tick began.
//
// The simulation moves in whole ticks and a frame lands between two of them.
// Reading e.X[i] straight into the draw therefore shows the world in
// tick-sized steps — a frame that carries two ticks or none is a pop of about
// two pixels at the default speed, which is a third of the frames on a 144 Hz
// display. Blending is what spends the frame's own position on it.
//
// It costs up to one tick of lag, because it draws between two positions the
// simulation has already been rather than guessing at one it has not. A guess
// is wrong every time something turns around, and a game full of wrong guesses
// is worse than a game a sixtieth of a second behind.
//
// It is the drawn position and nothing else. Collision, the camera's bounds
// and every rule read e.X[i], which is where the simulation actually is. A
// game drawing its own thing over an entity — a health bar, a name — wants
// this one, or it trails the sprite it is labelling by a tick.
//
// It panics when i is outside the arrays, which is a programmer error.
func (e *Engine) DrawPos(i int) (x, y float64) {
	t := e.tickBlend()
	// A weighted sum rather than prev+(x-prev)*t, because this one gives back
	// exactly the two ends at t of 0 and 1 however the rounding falls.
	return e.X[i]*t + e.PrevX[i]*(1-t), e.Y[i]*t + e.PrevY[i]*(1-t)
}

// tickBlend is how far this frame sits between the tick that has run and the
// one that has not: 0 the instant a tick ends, approaching 1 just before the
// next begins. With [RenderSettings.Interpolate] off it is 1, so DrawPos hands
// back the simulation's own position and the draw has one path either way.
func (e *Engine) tickBlend() float64 {
	if !e.Render.Interpolate {
		return 1
	}
	return min(max(e.tickAccum/e.tickStep(), 0), 1)
}

// tickStep is one tick in milliseconds. A rate of zero or less would divide by
// zero, so it falls back to the 60 a second [Defaults] sets.
func (e *Engine) tickStep() float64 {
	if e.Time.TickRate <= 0 {
		return 1000.0 / 60
	}
	return 1000 / e.Time.TickRate
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

// allowFullscreen reports whether a fullscreen toggle may happen now, and
// records the time when it may. Browsers refuse a burst of requests, so two
// toggles closer together than [TimeSettings.FullscreenDebounce] are one. The
// browser half calls it; it lives here so the rule can be tested off a
// browser, where there is no fullscreen to ask for.
func (e *Engine) allowFullscreen(now float64) bool {
	if now-e.fullscreenAt < e.Time.FullscreenDebounce {
		return false
	}
	e.fullscreenAt = now
	return true
}
