//go:build js && wasm

// The browser half of the engine: the canvas, images, sounds, input events,
// and the frame loop. Every syscall/js call in the module is in this file, so
// swapping Canvas2D for WebGL2 later rewrites one file and nothing else.

package wisp

import (
	"math"
	"syscall/js"
)

// backend holds the browser objects one engine drew with. It is a struct
// rather than an interface on purpose: there is one renderer, and an interface
// with one implementation is an abstraction layer with nothing on the other
// side of it.
type backend struct {
	assetVersion string
	canvas       js.Value
	ctx          js.Value
	doc          js.Value
	perf         js.Value

	images       []js.Value
	imagesLoaded int
	sounds       [][]js.Value
	soundsLoaded int
	nextVoice    []int
	music        js.Value

	drawn int
	funcs []js.Func
	loop  js.Func
}

// begin clears the canvas and puts the context back into the state the
// settings ask for.
//
// The smoothing flag is set every frame, not once at startup, because setting
// a canvas's width or height resets the whole 2D context — so a once-only
// version silently stops working after the first fullscreen toggle. One
// property assignment a frame costs nothing.
func (r *backend) begin(e *Engine) {
	r.drawn = 0
	r.ctx.Set("imageSmoothingEnabled", e.Render.Smoothing)
	if e.Render.Background == "" {
		r.ctx.Call("clearRect", 0, 0, e.Width, e.Height)
		return
	}
	r.ctx.Set("fillStyle", e.Render.Background)
	r.ctx.Call("fillRect", 0, 0, e.Width, e.Height)
}

// drawEntities paints the world under the camera, then the screen-space
// entities over it.
func (r *backend) drawEntities(e *Engine) {
	// The camera moves in whole pixels, shake included, or tile edges show
	// seams between them.
	x, y := -e.CamX+e.CamShakeX, -e.CamY+e.CamShakeY
	if e.Camera.Snap {
		x, y = math.Round(x), math.Round(y)
	}
	r.ctx.Call("save")
	r.ctx.Call("translate", x, y)
	r.pass(e, false)
	r.ctx.Call("restore")
	r.pass(e, true)
}

func (r *backend) end() {}

// now is the page's own monotonic clock, in milliseconds.
func (r *backend) now() float64 { return r.perf.Call("now").Float() }

// loadImages starts loading each image. A failure counts too, so one missing
// file can no longer leave the engine on "Loading..." for ever.
func (r *backend) loadImages(e *Engine, paths []string) {
	for _, path := range paths {
		img := js.Global().Get("Image").New()
		img.Set("onload", r.keep(func(js.Value, []js.Value) any {
			r.imagesLoaded++
			return nil
		}))
		img.Set("onerror", r.keep(func(js.Value, []js.Value) any {
			r.imagesLoaded++
			e.failed = append(e.failed, path)
			return nil
		}))
		img.Set("src", r.assetURL(path))
		r.images = append(r.images, img)
	}
}

// loadSounds starts loading each sound, one Audio element per voice, so a
// re-trigger takes the next one instead of being dropped.
func (r *backend) loadSounds(e *Engine, paths []string) {
	for _, path := range paths {
		voices := make([]js.Value, max(e.Audio.Voices, 1))
		for i := range voices {
			a := js.Global().Get("Audio").New()
			if i == 0 {
				// Only the first voice reports: the others are copies of the
				// same file and would count the same arrival many times.
				a.Set("oncanplaythrough", r.keep(func(js.Value, []js.Value) any {
					r.soundsLoaded++
					return nil
				}))
				a.Set("onerror", r.keep(func(js.Value, []js.Value) any {
					r.soundsLoaded++
					e.failed = append(e.failed, path)
					return nil
				}))
			}
			a.Set("src", r.assetURL(path))
			voices[i] = a
		}
		r.sounds = append(r.sounds, voices)
		r.nextVoice = append(r.nextVoice, 0)
	}
}

func (r *backend) loading() bool {
	return r.imagesLoaded < len(r.images) || r.soundsLoaded < len(r.sounds)
}

func (r *backend) pauseMusic() {
	if r.music.Truthy() && !r.music.Get("paused").Bool() {
		r.music.Call("pause")
	}
}

// playMusic starts a loop, or does nothing when it is already running, so a
// game may call it every frame.
func (r *backend) playMusic(index int, volume float64) {
	if index < 0 || index >= len(r.sounds) {
		return
	}
	a := r.sounds[index][0]
	if !a.Truthy() || !a.Get("paused").Bool() {
		return
	}
	a.Set("loop", true)
	a.Set("volume", clamp01(volume))
	a.Call("play")
	r.music = a
}

// playSound takes the next voice, so three monsters dying in one frame make
// three sounds. One Audio element per sound is what made that one sound.
func (r *backend) playSound(index int, volume, rate float64) {
	if index < 0 || index >= len(r.sounds) {
		return
	}
	voices := r.sounds[index]
	a := voices[r.nextVoice[index]]
	r.nextVoice[index] = (r.nextVoice[index] + 1) % len(voices)
	if !a.Truthy() {
		return
	}
	a.Set("currentTime", 0)
	a.Set("loop", false)
	a.Set("playbackRate", rate)
	a.Set("volume", clamp01(volume))
	a.Call("play")
}

func (r *backend) rect(x, y, w, h float64, color string) {
	r.ctx.Set("fillStyle", color)
	r.ctx.Call("fillRect", x, y, w, h)
}

// run creates the canvas inside the mount element and starts the frame loop.
// It blocks until Stop, so a game's main is one call.
func (r *backend) run(e *Engine) {
	r.doc = js.Global().Get("document")
	r.perf = js.Global().Get("performance")

	r.canvas = r.doc.Call("createElement", "canvas")
	r.canvas.Set("width", e.Width)
	r.canvas.Set("height", e.Height)
	mount := r.doc.Call("querySelector", "main")
	if !mount.Truthy() {
		mount = r.doc.Get("body")
	}
	mount.Call("appendChild", r.canvas)
	r.ctx = r.canvas.Call("getContext", "2d")

	r.listen(e)

	last := r.perf.Call("now").Float()
	done := make(chan struct{})

	// The callback is kept on the runtime so the garbage collector does not
	// free it while the browser still holds it.
	r.loop = js.FuncOf(func(js.Value, []js.Value) any {
		now := r.perf.Call("now").Float()
		e.Step(now - last)
		last = now
		e.draw()
		if e.stopped {
			close(done)
			return nil
		}
		js.Global().Call("requestAnimationFrame", r.loop)
		return nil
	})
	js.Global().Call("requestAnimationFrame", r.loop)
	<-done
}

// stop releases every callback the browser holds.
func (r *backend) stop() {
	for _, f := range r.funcs {
		f.Release()
	}
	r.funcs = nil
}

func (r *backend) stopMusic() {
	if r.music.Truthy() {
		r.music.Set("currentTime", 0)
		r.music.Call("pause")
	}
}

func (r *backend) storeGet(key string) string {
	s := js.Global().Get("localStorage")
	if !s.Truthy() {
		return ""
	}
	v := s.Call("getItem", key)
	if !v.Truthy() {
		return ""
	}
	return v.String()
}

func (r *backend) storeSet(key, value string) {
	if s := js.Global().Get("localStorage"); s.Truthy() {
		s.Call("setItem", key, value)
	}
}

func (r *backend) text(x, y float64, text, color, font, align string) {
	r.ctx.Set("fillStyle", color)
	r.ctx.Set("font", font)
	r.ctx.Set("textAlign", align)
	r.ctx.Set("textBaseline", "middle")
	r.ctx.Call("fillText", text, x, y)
}

// copyText puts the tuned settings on the clipboard. It needs a secure
// context, which http://127.0.0.1 is, so `make run` is enough.
func (r *backend) copyText(s string) {
	clip := js.Global().Get("navigator").Get("clipboard")
	if !clip.Truthy() {
		return
	}
	clip.Call("writeText", s)
}

// assetURL adds the build version to an asset path. Static assets are served
// with a one-year immutable cache, so the version in the URL is what makes a
// browser fetch a new sheet after a rebuild. The page puts it in data-version
// on the html element.
func (r *backend) assetURL(path string) string {
	if r.assetVersion == "" {
		v := js.Global().Get("document").Get("documentElement").Get("dataset").Get("version")
		if !v.Truthy() {
			return path
		}
		r.assetVersion = v.String()
	}
	return path + "?v=" + r.assetVersion
}

// keep registers a callback and holds on to it, so the garbage collector does
// not free it while the browser still holds a reference.
func (r *backend) keep(fn func(js.Value, []js.Value) any) js.Func {
	f := js.FuncOf(fn)
	r.funcs = append(r.funcs, f)
	return f
}

// listen wires the keyboard on the window and the mouse on the canvas.
func (r *backend) listen(e *Engine) {
	on := func(target js.Value, event string, fn func(js.Value)) {
		target.Call("addEventListener", event, r.keep(func(_ js.Value, args []js.Value) any {
			if len(args) > 0 && args[0].Truthy() {
				fn(args[0])
			}
			return nil
		}))
	}
	win := js.Global()

	on(win, "keydown", func(ev js.Value) {
		// Auto-repeat is not a new press, and treating it as one turns a held
		// key into a machine gun in every JustPressed branch.
		if ev.Get("repeat").Bool() {
			return
		}
		key := ev.Get("key").String()
		e.Input.SetModifiers(ev.Get("altKey").Bool(), ev.Get("ctrlKey").Bool(), ev.Get("metaKey").Bool(), ev.Get("shiftKey").Bool())
		e.Input.Key(key, true)
		if e.claimsKey(key) {
			// Only keys the engine claims are swallowed. A preventDefault on
			// everything steals the browser's own shortcuts, and one on
			// mousemove kills text selection across the whole page.
			ev.Call("preventDefault")
		}
		if e.fullscreenKey != KeyNone && normalizeKey(key) == e.fullscreenKey {
			r.toggleFullscreen(e)
		}
	})
	on(win, "keyup", func(ev js.Value) {
		e.Input.SetModifiers(ev.Get("altKey").Bool(), ev.Get("ctrlKey").Bool(), ev.Get("metaKey").Bool(), ev.Get("shiftKey").Bool())
		e.Input.Key(ev.Get("key").String(), false)
	})
	on(r.canvas, "mousedown", func(ev js.Value) {
		e.Input.MouseDown = true
		e.Input.Started = true
		ev.Call("preventDefault")
	})
	on(r.canvas, "mouseup", func(js.Value) { e.Input.MouseDown = false })
	on(r.canvas, "mousemove", func(ev js.Value) {
		// The canvas is scaled by CSS, so map the pointer back to canvas
		// pixels, then into the world. The shake does not move it: a click
		// must land where the player aimed, not where the screen wobbled to.
		rect := r.canvas.Call("getBoundingClientRect")
		sx := e.Width / rect.Get("width").Float()
		sy := e.Height / rect.Get("height").Float()
		e.Input.MouseX = (ev.Get("clientX").Float()-rect.Get("left").Float())*sx + e.CamX
		e.Input.MouseY = (ev.Get("clientY").Float()-rect.Get("top").Float())*sy + e.CamY
	})
}

// pass draws one layer: world space under the camera transform, or screen
// space after it. Entities outside the view are skipped.
func (r *backend) pass(e *Engine, screenSpace bool) {
	vw, vh := e.Width, e.Height
	vLeft, vTop := 0.0, 0.0
	if !screenSpace {
		vLeft, vTop = e.CamX-e.CamShakeX, e.CamY-e.CamShakeY
	}
	vRight, vBottom := vLeft+vw, vTop+vh

	alpha := 1.0
	for _, i := range e.drawOrder {
		if e.ScreenSpace[i] != screenSpace || e.State[i]&StateVisible == 0 {
			continue
		}
		img := r.images[e.ImageIndex[i]]
		if !img.Truthy() {
			continue
		}

		w, h := e.SpriteWidth[i], e.SpriteHeight[i]
		dstX := e.X[i] - w/2
		dstY := e.Y[i] - h/2
		if dstX+w < vLeft || dstX > vRight || dstY+h < vTop || dstY > vBottom {
			continue
		}
		if e.Render.PixelSnap {
			dstX, dstY = math.Round(dstX), math.Round(dstY)
		}

		// The column picks the sprite and the frame offset picks the frame, so
		// one sheet holds every animation of every sprite.
		srcX := float64(e.ImageColumn[i]+e.FrameOffset[i]) * w
		srcY := float64(e.ImageRow[i]) * h

		if e.Alpha[i] != alpha {
			alpha = e.Alpha[i]
			r.ctx.Set("globalAlpha", alpha)
		}
		r.ctx.Call("drawImage", img, srcX, srcY, w, h, dstX, dstY, w, h)
		r.drawn++
	}
	if alpha != 1.0 {
		r.ctx.Set("globalAlpha", 1.0)
	}
}

// toggleFullscreen enters or leaves fullscreen, no more often than the
// debounce allows: browsers refuse a burst of requests.
func (r *backend) toggleFullscreen(e *Engine) {
	now := r.perf.Call("now").Float()
	if now-e.fullscreenAt < e.Time.FullscreenDebounce {
		return
	}
	e.fullscreenAt = now

	for _, name := range []string{"fullscreenElement", "webkitFullscreenElement"} {
		if el := r.doc.Get(name); el.Truthy() && el.Equal(r.canvas) {
			for _, exit := range []string{"exitFullscreen", "webkitExitFullscreen"} {
				if r.doc.Get(exit).Truthy() {
					r.doc.Call(exit)
					return
				}
			}
			return
		}
	}
	for _, enter := range []string{"requestFullscreen", "webkitRequestFullscreen"} {
		if r.canvas.Get(enter).Truthy() {
			r.canvas.Call(enter)
			return
		}
	}
}

// clamp01 keeps a volume inside the range the Audio element accepts. It throws
// on anything else, which would take the frame loop down with it.
func clamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return min(max(v, 0), 1)
}

// hostStats fills in the numbers only the page can answer. The loader
// publishes them on globalThis.wisp; Go only ever reads that object.
//
// Two rows stay empty on purpose. No browser exposes processor load, and none
// exposes graphics-card load — the timer-query extension that would give the
// card's own frame time is switched off in Chrome for almost everyone. The
// overlay prints "n/a" for both rather than a number nobody could act on.
func (r *backend) hostStats(s *Stats) {
	w := js.Global().Get("wisp")
	if !w.Truthy() {
		s.GPU = "n/a"
		s.LongFrames = -1
		return
	}

	s.WasmBytes = w.Get("wasmBytes").Int()

	// The buffer is read fresh every time: growing the memory detaches the old
	// one, so a cached handle would report the size it had before the growth.
	if mem := w.Get("memory"); mem.Truthy() {
		s.WasmMemoryBytes = uint64(mem.Get("buffer").Get("byteLength").Int())
	}

	if gpu := w.Get("gpu"); gpu.Truthy() {
		s.GPU = gpu.Get("renderer").String()
		if gpu.Get("masked").Bool() {
			s.GPU += " (masked)"
		}
	}

	// performance.memory is Chromium's alone and is not standard, so the
	// overlay says n/a elsewhere rather than pretending the heap is empty.
	if pm := js.Global().Get("performance").Get("memory"); pm.Truthy() {
		s.JSHeapBytes = uint64(pm.Get("usedJSHeapSize").Int())
	}

	// long-animation-frame sees style, layout and paint, so it notices a slow
	// canvas; longtask only sees script. Neither exists outside Chromium.
	if w.Get("longFramesKind").String() == "none" {
		s.LongFrames = -1
	} else {
		s.LongFrames = w.Get("longFrames").Int()
	}
}
