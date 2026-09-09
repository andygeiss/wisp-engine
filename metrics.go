package wisp

import (
	"runtime"
	"slices"
	"strconv"
)

// metricsWindow is how many frames the statistics remember: two seconds at 60
// frames a second, which is long enough for a percentile to mean something and
// short enough to react while you are still holding the key.
const metricsWindow = 120

// lateFrame is how many budgets a frame has to take before it counts as late.
// The budget is the middle of the window, so about half of a healthy scene's
// frames sit just above it: comparing against the budget itself marks a steady
// 60 fps as a problem, which is what the frame row and the sparkline used to
// do. One and a half budgets is a frame the display actually missed.
const lateFrame = 1.5

// Stats is what the last two seconds cost. [Engine.Stats] returns it, and the
// overlay draws it.
//
// Three numbers a person asks for are missing, and they are missing because no
// browser offers them: processor load, graphics-card load, and the graphics
// card's own frame time. What is here instead is the honest neighbour of each —
// how much of the frame budget the main thread spent, and how much work was
// asked of the renderer.
type Stats struct {
	// Budget is one frame's worth of milliseconds, measured from the display
	// rather than assumed: 16.7 at 60 Hz, 8.3 at 120.
	Budget float64
	// DrawMs is how long the last frame spent drawing.
	DrawMs float64
	// Drawn is how many sprites the last frame actually painted, after the
	// camera threw the off-screen ones away.
	Drawn int
	// Dropped counts frames that took more than one and a half budgets.
	Dropped int
	// Entities is how many entities exist, drawn or not.
	Entities int
	// FPS is the frame rate over the window.
	FPS float64
	// FrameMs is the last frame, wall to wall.
	FrameMs float64
	// GoHeapBytes is what the Go heap holds. Under TinyGo this is a subset of
	// what ordinary Go reports; the overlay says so rather than guessing.
	GoHeapBytes uint64
	// GPU names the graphics card, or says why it cannot.
	GPU string
	// JSHeapBytes is the JavaScript heap. Chromium only; 0 elsewhere.
	JSHeapBytes uint64
	// Load is the share of the frame budget the main thread spent working,
	// 0 to 1 and beyond. It is the closest honest thing to processor load.
	Load float64
	// Late is the frame time, in milliseconds, at which a frame counts as
	// missed: [Stats.Budget] times a tolerance. Compare a percentile against
	// this rather than against Budget, which half of a healthy window exceeds
	// by construction. 0 when the budget is not known yet.
	Late float64
	// LongFrames counts frames the browser itself reported as too long.
	// Chromium only; -1 where the browser does not offer it.
	LongFrames int
	// P50Ms and P99Ms are the middle and the near-worst frame in the window.
	// P99 is the number that decides whether a game feels smooth; an average
	// hides exactly the frames a player notices.
	P50Ms float64
	P99Ms float64
	// SortMs is how long the last frame spent putting the entities in draw
	// order. It is separate from [Stats.DrawMs] because the two answer
	// different questions: the sort is a function of how many entities exist
	// and a different renderer would not change it, while the draw is what a
	// different renderer would replace.
	SortMs float64
	// UpdateMs is how long the last frame spent on the world.
	UpdateMs float64
	// WasmBytes is the module's own size, which is the number this engine
	// exists to keep small.
	WasmBytes int
	// WasmMemoryBytes is the WebAssembly linear memory the module holds. It
	// only ever grows, and it grows in 64 KiB pages.
	WasmMemoryBytes uint64
}

// metrics collects the numbers as the frames go by. Everything it holds is a
// fixed-size array, so a frame measures itself without allocating.
type metrics struct {
	frames  [metricsWindow]float64
	scratch [metricsWindow]float64
	n       int
	next    int

	drawMs   float64
	drawn    int
	dropped  int
	sortMs   float64
	updateMs float64

	// refresh is the display's own frame interval: the smallest window median
	// seen so far. See budget.
	refresh float64

	heapAt      float64
	goHeapBytes uint64
}

// add records one frame's wall time.
func (m *metrics) add(ms float64) {
	m.frames[m.next] = ms
	m.next = (m.next + 1) % metricsWindow
	m.n = min(m.n+1, metricsWindow)
	// The display's interval is the lowest this window's middle has ever
	// been: the scene is at its lightest somewhere, and nothing a game does
	// makes the hardware faster. Taking the middle rather than the single
	// fastest frame is what makes it safe to keep — one coalesced callback
	// would latch a two-millisecond "display" for the rest of the run, and
	// every frame after it would count as late.
	if m.n >= metricsWindow/4 {
		if mid := m.percentile(0.5); m.refresh == 0 || mid < m.refresh {
			m.refresh = mid
		}
	}
	if b := m.budget(); b > 0 && ms > b*lateFrame {
		m.dropped++
	}
}

// budget is one frame's worth of milliseconds. Measuring it beats assuming
// 16.7: a 120 Hz display has half of that, and a load figure against the wrong
// budget is worse than none.
//
// It is the lowest the window's middle has been, not the middle of the window
// now. The middle now measures the display only while the scene keeps up: at
// twenty thousand sprites every frame is slow and the middle rises with them,
// so slow becomes the new normal and nothing can ever be late again. The
// budget is a property of the hardware, so it may fall as the true interval is
// learned and must never rise with the load.
func (m *metrics) budget() float64 { return m.refresh }

// percentile returns the frame at p through the window, sorted. It sorts a
// fixed scratch array in place, so it allocates nothing however often it runs.
func (m *metrics) percentile(p float64) float64 {
	if m.n == 0 {
		return 0
	}
	copy(m.scratch[:m.n], m.frames[:m.n])
	s := m.scratch[:m.n]
	slices.Sort(s)
	i := int(p * float64(m.n-1))
	return s[min(max(i, 0), m.n-1)]
}

// Stats returns what the last two seconds cost.
func (e *Engine) Stats() Stats {
	m := &e.metrics
	s := Stats{
		Budget:      m.budget(),
		Late:        m.budget() * lateFrame,
		SortMs:      m.sortMs,
		DrawMs:      m.drawMs,
		Drawn:       m.drawn,
		Dropped:     m.dropped,
		Entities:    e.live,
		FrameMs:     m.frames[(m.next-1+metricsWindow)%metricsWindow],
		GoHeapBytes: m.goHeapBytes,
		P50Ms:       m.percentile(0.5),
		P99Ms:       m.percentile(0.99),
		UpdateMs:    m.updateMs,
	}
	if avg := m.mean(); avg > 0 {
		s.FPS = 1000 / avg
	}
	if s.Budget > 0 {
		s.Load = (m.updateMs + m.sortMs + m.drawMs) / s.Budget
	}
	e.rt.hostStats(&s)
	return s
}

// mean is the window's average frame, which is what a frame rate is.
func (m *metrics) mean() float64 {
	if m.n == 0 {
		return 0
	}
	total := 0.0
	for _, v := range m.frames[:m.n] {
		total += v
	}
	return total / float64(m.n)
}

// sampleHeap reads the Go heap, once a second rather than once a frame.
//
// Reading it is not free: TinyGo's ReadMemStats walks the whole block
// metadata to count what is live, so a per-frame read would show up in the
// very frame time it is meant to explain. HeapAlloc is one of the fields it
// really fills — the wasm target builds the gc_blocks collector, which assigns
// it — so the row is a measurement rather than a constant.
func (m *metrics) sampleHeap(elapsed float64) {
	m.heapAt += elapsed
	if m.heapAt < 1000 && m.n > 0 {
		return
	}
	m.heapAt = 0
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	m.goHeapBytes = ms.HeapAlloc
}

// The overlay's own measurements, in canvas pixels.
const (
	hudFont  = "11px ui-monospace, monospace"
	hudPad   = 6.0
	hudRowH  = 13.0
	hudWidth = 268.0
	// hudBars is how many bars the frame-time graph draws. One bar per frame
	// would be 120 crossings into JavaScript every frame, for a graph nobody
	// can read at that width anyway.
	hudBars = 60
)

// drawMetrics paints the overlay. It is drawn with the same two primitives as
// everything else, so it is inside the canvas in fullscreen too.
func (e *Engine) drawMetrics() {
	if !e.Debug.ShowMetrics {
		return
	}
	const (
		ground = "rgba(0, 0, 0, 0.72)"
		good   = "rgba(120, 220, 140, 0.9)"
		warn   = "rgba(240, 200, 90, 0.9)"
		bad    = "rgba(240, 110, 110, 0.9)"
		dim    = "rgba(255, 255, 255, 0.55)"
		plain  = "white"
	)
	s := e.Stats()

	rows := 8
	h := float64(rows)*hudRowH + hudPad*3 + 26
	e.Rect(0, 0, hudWidth, h, ground)

	y := hudPad + hudRowH/2
	line := func(label, value, color string) {
		e.Text(hudPad, y, label, dim, hudFont, "left")
		e.Text(hudWidth-hudPad, y, value, color, hudFont, "right")
		y += hudRowH
	}

	frameColor := good
	switch {
	case s.Budget > 0 && s.P99Ms > s.Budget*2:
		frameColor = bad
	case s.Late > 0 && s.P99Ms > s.Late:
		frameColor = warn
	}

	line("fps", formatMillis(s.FPS), plain)
	line("frame  p50 / p99", formatMillis(s.P50Ms)+" / "+formatMillis(s.P99Ms)+" ms", frameColor)
	line("update", formatMillis(s.UpdateMs)+" ms", plain)
	// Two numbers rather than one: a renderer swap moves the draw and leaves
	// the sort exactly where it is, and that is the whole WebGL2 question.
	line("sort / draw", formatMillis(s.SortMs)+" / "+formatMillis(s.DrawMs)+" ms", plain)
	line("main thread", formatPercent(s.Load)+"  of "+formatMillis(s.Budget)+" ms", plain)
	line("entities / drawn", itoa(s.Entities)+" / "+itoa(s.Drawn), plain)
	line("go heap / wasm mem", formatBytes(s.GoHeapBytes)+" / "+formatBytes(s.WasmMemoryBytes), plain)
	line("cpu / gpu load", "n/a — not exposed by browsers", dim)

	y += hudPad / 2
	e.Text(hudPad, y, "gpu "+s.GPU, dim, hudFont, "left")
	y += hudRowH

	// The frame-time graph. Bars are merged by colour, so a steady scene costs
	// three fills rather than sixty.
	e.drawSparkline(hudPad, y, hudWidth-hudPad*2, 20, s.Budget, good, warn, bad)
}

// drawSparkline paints the frame window as bars, newest on the right. A bar
// past the budget is amber and one past twice it is red, so a stutter is
// visible without reading a number.
//
// It is one fill per bar. Merging runs of one colour would be fewer calls into
// the browser, but every bar has its own height, and the heights are the
// reason to draw a graph at all. Sixty fills is what the overlay costs, and
// the overlay reports its own cost.
func (e *Engine) drawSparkline(x, y, w, h, budget float64, good, warn, bad string) {
	m := &e.metrics
	if m.n == 0 || budget <= 0 {
		return
	}
	bw := w / hudBars
	scale := h / (budget * 3) // three budgets fills the graph

	for i := range hudBars {
		// Walk back from the newest frame, so the right-hand edge is now.
		idx := (m.next - 1 - (hudBars - 1 - i) + metricsWindow*2) % metricsWindow
		v := m.frames[idx]
		if v <= 0 {
			continue
		}
		color := good
		switch {
		case v > budget*2:
			color = bad
		case v > budget*lateFrame:
			color = warn
		}
		bh := min(v*scale, h)
		e.Rect(x+float64(i)*bw, y+h-bh, bw, bh, color)
	}
}

// formatBytes spells a size without reaching for fmt, which costs a TinyGo
// build tens of kilobytes of reflection for one line of text.
func formatBytes(n uint64) string {
	switch {
	case n == 0:
		return "n/a"
	case n < 1024:
		return strconv.FormatUint(n, 10) + " B"
	case n < 1024*1024:
		return strconv.FormatFloat(float64(n)/1024, 'f', 1, 64) + " K"
	}
	return strconv.FormatFloat(float64(n)/(1024*1024), 'f', 1, 64) + " M"
}

// formatMillis spells a duration with one decimal.
func formatMillis(v float64) string {
	if v != v { // NaN
		return "n/a"
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// formatPercent spells a share of the budget as a whole percentage.
func formatPercent(v float64) string {
	if v != v {
		return "n/a"
	}
	return strconv.FormatFloat(v*100, 'f', 0, 64) + "%"
}

// itoa spells a whole number. strconv is a few hundred bytes; fmt is tens of
// thousands.
func itoa(n int) string { return strconv.Itoa(n) }
