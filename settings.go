package wisp

// Settings is every knob that decides how a game feels. One value, one place:
// [Defaults] fills it, the menu edits it while the game runs, and
// [Settings.GoLiteral] hands it back as Go source to paste into your game.
//
// Every field is a number or a bool, which is what lets one menu edit all of
// them without knowing what any of them mean.
type Settings struct {
	Animation AnimationSettings
	Audio     AudioSettings
	Camera    CameraSettings
	Debug     DebugSettings
	Feel      FeelSettings
	Render    RenderSettings
	Time      TimeSettings
	World     WorldSettings
}

// AnimationSettings decides how sprite animations run when a sheet does not
// say otherwise.
type AnimationSettings struct {
	// CullOffscreen stops an entity outside the view from advancing its
	// animation. It saves work in a crowded world, and it means an enemy walks
	// on screen mid-stride instead of from frame zero.
	CullOffscreen bool
	// FrameCount is how many frames a grid sheet puts in one row.
	FrameCount int
	// FrameDuration is how long one frame lasts, in milliseconds.
	FrameDuration float64
}

// AudioSettings decides how loud things are and how much they vary.
type AudioSettings struct {
	// MusicVolume scales the music, on top of Volume.
	MusicVolume float64
	// PitchJitter randomises playback speed by up to this fraction, so the
	// same hit sound does not sound identical twenty times in a row.
	PitchJitter float64
	// SfxVolume scales the sound effects, on top of Volume.
	SfxVolume float64
	// Voices is how many copies of one sound may play at once. One copy means
	// three monsters dying together make one sound.
	Voices int
	// Volume scales everything.
	Volume float64
}

// CameraSettings decides how the camera follows its target.
type CameraSettings struct {
	// DeadzoneHeight is how far the target may move up or down before the
	// camera follows, in pixels.
	DeadzoneHeight float64
	// DeadzoneWidth is the same across.
	DeadzoneWidth float64
	// Lookahead pushes the camera this many pixels ahead of the way the target
	// is moving, so you see where you are going.
	Lookahead float64
	// Smoothing is how fast the camera catches up with its target, per
	// second: 0 snaps to it, 8 is a soft follow, 30 is almost a snap. The
	// engine closes the gap by 1-exp(-Smoothing*dt), so the feel does not
	// change between a 60 Hz screen and a 144 Hz one.
	Smoothing float64
	// Snap rounds the camera to whole pixels. Off, tile edges show seams.
	Snap bool
}

// DebugSettings turns the engine's own overlays on.
type DebugSettings struct {
	// ShowHitBoxes draws every entity's collision box.
	ShowHitBoxes bool
	// ShowMetrics draws the frame-time and memory overlay.
	ShowMetrics bool
}

// FeelSettings is the impact of a hit, in three strengths a game picks from,
// plus the shake curve all three share.
//
// A game says which strength something is worth — e.Impact(e.Feel.Heavy) —
// and never writes a number at the call site. The numbers live here, where
// the menu can reach them.
type FeelSettings struct {
	Heavy  ImpactSettings
	Light  ImpactSettings
	Medium ImpactSettings
	Shake  ShakeSettings
}

// ImpactSettings is one hit's worth of feel: how long time stops, and how hard
// the screen shakes.
type ImpactSettings struct {
	// HitStopDuration is how long the world holds still, in milliseconds.
	HitStopDuration float64
	// HitStopScale is how fast time runs while it holds: 0 is a freeze, 0.1 is
	// slow motion, 1 does nothing.
	HitStopScale float64
	// ShakeDuration is how long the shake lasts, in milliseconds.
	ShakeDuration float64
	// ShakeMagnitude is how far the camera may be thrown, in pixels.
	ShakeMagnitude float64
}

// ShakeSettings shapes every shake, whatever set it off.
type ShakeSettings struct {
	// Decay is the shape of the fade: 1 falls off in a straight line, 2 drops
	// fast then lingers, 0.5 holds then stops.
	Decay float64
	// Frequency is how many times a second the camera is thrown somewhere new.
	// Below the frame rate it reads as a rumble, above it as noise.
	Frequency float64
	// IgnoresHitStop keeps the shake moving while a hit stop holds the world
	// still, so a freeze still feels violent.
	IgnoresHitStop bool
	// Snap rounds the shake to whole pixels, like the camera.
	Snap bool
}

// RenderSettings decides how the picture is drawn.
type RenderSettings struct {
	// Background is the CSS colour the canvas is cleared to. Empty clears to
	// transparent and lets the page show through.
	Background string
	// Interpolate draws each entity between the tick that has run and the one
	// that has not, so a frame carrying two ticks or none does not show a pop.
	// It costs the draw a little arithmetic and shows the world up to one tick
	// in the past. Off, every sprite sits exactly where the simulation put it,
	// and moves in the steps the simulation moved it in.
	//
	// It smooths what [Engine.Simulate] moves. An entity moved from the update
	// [Engine.Run] was given is already frame-accurate, and blending drags it
	// backwards instead — which is one more reason movement belongs in a tick.
	Interpolate bool
	// PixelSnap rounds sprite positions to whole pixels.
	PixelSnap bool
	// Smoothing turns on the canvas's own interpolation. Pixel art wants it
	// off; leaving it on is what makes a scaled sprite look blurry.
	Smoothing bool
}

// TimeSettings decides how fast the world runs.
type TimeSettings struct {
	// FullscreenDebounce is the shortest gap, in milliseconds, between two
	// fullscreen toggles. Browsers refuse a burst of requests.
	FullscreenDebounce float64
	// MaxStep caps one frame's elapsed time, in milliseconds. Without it a tab
	// left in the background teleports everything when it comes back.
	MaxStep float64
	// Scale multiplies every frame's elapsed time. Below 1 is slow motion.
	Scale float64
	// TickRate is how many times a second the simulation runs, apart from the
	// frame rate. A higher rate costs more and reacts sooner; a lower one is
	// cheaper and coarser.
	//
	// Two machines simulating one world have to agree on it, so a networked
	// game sets it from the server rather than from a saved menu.
	TickRate float64
}

// WorldSettings decides how entities move and touch.
type WorldSettings struct {
	// HitBoxMargin shrinks every hit box by this many pixels on all four
	// sides, so sprites have to visibly overlap before they collide.
	HitBoxMargin float64
	// Speed is the base movement speed, in pixels per millisecond.
	Speed float64
}

// Defaults returns the settings a new engine starts with. They are the
// numbers the game in game-jam-template was balanced around, so a game that
// changes nothing plays the way that one does.
func Defaults() Settings {
	return Settings{
		Animation: AnimationSettings{
			CullOffscreen: false,
			FrameCount:    8,
			FrameDuration: 100,
		},
		Audio: AudioSettings{
			MusicVolume: 0.25,
			PitchJitter: 0,
			SfxVolume:   1,
			Voices:      4,
			Volume:      1,
		},
		Camera: CameraSettings{
			DeadzoneHeight: 0,
			DeadzoneWidth:  0,
			Lookahead:      0,
			Smoothing:      0,
			Snap:           true,
		},
		Debug: DebugSettings{},
		Feel: FeelSettings{
			// A light hit is a connect, a heavy one is a finisher. The
			// spacing between them is what a player reads as weight.
			Heavy:  ImpactSettings{HitStopDuration: 150, ShakeDuration: 400, ShakeMagnitude: 10},
			Light:  ImpactSettings{HitStopDuration: 70, ShakeDuration: 100, ShakeMagnitude: 2.5},
			Medium: ImpactSettings{HitStopDuration: 100, ShakeDuration: 150, ShakeMagnitude: 4},
			Shake: ShakeSettings{
				Decay:          2,
				Frequency:      30,
				IgnoresHitStop: true,
				Snap:           true,
			},
		},
		Render: RenderSettings{
			Interpolate: true,
			PixelSnap:   true,
			Smoothing:   false,
		},
		Time: TimeSettings{
			FullscreenDebounce: 500,
			MaxStep:            50,
			Scale:              1,
			TickRate:           60,
		},
		World: WorldSettings{
			HitBoxMargin: 6,
			Speed:        0.125,
		},
	}
}
