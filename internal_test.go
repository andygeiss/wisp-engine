package wisp

import "testing"

// drawOrder is the one thing a consumer cannot reach, so its test lives here.
// Everything else is tested from package wisp_test, the way a game sees it.

func TestSortDrawOrder(t *testing.T) {
	t.Parallel()
	e := New(Config{})
	e.Add(Sprite{Height: 32, State: StateVisible, Width: 32, Y: 10, Z: 1})  // 0: z 1
	e.Add(Sprite{Height: 32, State: StateVisible, Width: 32, Y: 100, Z: 0}) // 1: z 0, low on screen
	e.Add(Sprite{Height: 32, State: StateVisible, Width: 32, Y: 50, Z: 0})  // 2: z 0, higher up
	e.Add(Sprite{Height: 32, State: StateVisible, Width: 32, Y: 50, Z: 0})  // 3: ties with 2

	e.sortDrawOrder()

	want := []int{2, 3, 1, 0}
	for i := range want {
		if e.drawOrder[i] != want[i] {
			t.Fatalf("drawOrder = %v, want %v", e.drawOrder, want)
		}
	}
}

func TestDeleteRebuildsDrawOrder(t *testing.T) {
	t.Parallel()
	e := New(Config{})
	// Z runs backwards so the sorted draw order is not the index order.
	for i, x := range []float64{10, 20, 30} {
		e.Add(Sprite{Height: 32, State: StateVisible, Width: 32, X: x, Z: 2 - i})
	}
	e.sortDrawOrder()

	e.Delete(1)

	if len(e.drawOrder) != 2 || e.drawOrder[0] != 1 || e.drawOrder[1] != 0 {
		t.Errorf("drawOrder = %v, want [1 0]", e.drawOrder)
	}
	for _, idx := range e.drawOrder {
		if idx < 0 || idx >= len(e.State) {
			t.Fatalf("drawOrder holds %d, outside 0..%d", idx, len(e.State)-1)
		}
	}
}

// The clipboard and the browser's storage have no consumer-visible surface, so
// what the menu writes to them is checked from inside the package.

func TestMenuCopiesGoSource(t *testing.T) {
	t.Parallel()
	e := New(Config{})
	e.Input.Key("m", true)
	e.Step(16)
	e.Input.Key("m", false)

	e.Input.Key("c", true)
	e.Step(16)

	got := e.rt.storeGet("clipboard")
	if got == "" {
		t.Fatal("C copied nothing")
	}
	if got != e.Settings.GoLiteral() {
		t.Errorf("the clipboard does not hold the settings literal:\n%s", got)
	}
}

func TestMenuRemembersTuning(t *testing.T) {
	t.Parallel()
	e := New(Config{})
	e.Input.Key("m", true)
	e.Step(16)
	e.Input.Key("m", false)

	e.World.Speed = 0.4
	e.menu.changed()
	// The save waits half a second after the last change, so a held arrow key
	// does not write on every frame. Step caps one frame at Time.MaxStep, so
	// half a second is ten frames, not one.
	for range int(saveDelay/e.Time.MaxStep) + 1 {
		e.Step(e.Time.MaxStep)
	}

	text := e.rt.storeGet(storageKey)
	if text == "" {
		t.Fatal("nothing was saved")
	}

	// A second engine sharing the same storage must come up tuned.
	next := New(Config{})
	next.rt = e.rt
	next.menu.load(next)
	if next.World.Speed != 0.4 {
		t.Errorf("Speed = %v after loading, want 0.4", next.World.Speed)
	}
}

func TestMenuOffWritesNothing(t *testing.T) {
	t.Parallel()
	e := New(Config{MenuKey: KeyNone})
	e.World.Speed = 0.4
	e.menu.changed()
	for range int(saveDelay/e.Time.MaxStep) + 1 {
		e.Step(e.Time.MaxStep)
	}

	if got := e.rt.storeGet(storageKey); got != "" {
		t.Errorf("a game with no menu wrote settings to the browser: %q", got)
	}
}

// The frame statistics are internal: a game sees Stats, not the ring buffer.
// These test the ring itself, because the percentile maths and the allocation
// discipline are what make the overlay trustworthy.

func TestPercentile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		samples []float64
		p       float64
		want    float64
	}{
		{"one sample is every percentile", []float64{7}, 0.99, 7},
		{"a flat window has one answer", flat(metricsWindow, 16), 0.5, 16},
		{"a ramp puts the middle in the middle", ramp(metricsWindow), 0.5, 59},
		{"a ramp puts the worst near the end", ramp(metricsWindow), 0.99, 117},
		{"p99 catches a stutter p50 misses", stutter(metricsWindow, 16, 200), 0.99, 200},
		{"p50 ignores that stutter", stutter(metricsWindow, 16, 200), 0.5, 16},
		{"a part-full window uses what it has", []float64{5, 1, 3}, 0.5, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var m metrics
			for _, v := range tt.samples {
				m.add(v)
			}
			if got := m.percentile(tt.p); got != tt.want {
				t.Errorf("percentile(%v) = %v, want %v", tt.p, got, tt.want)
			}
		})
	}
}

// TestMetricsAllocFree is the discipline this engine lives by, written as a
// gate. A frame that allocates to measure itself is a frame that measures its
// own garbage collection.
//
// It is the one test here that does not run in parallel: AllocsPerRun counts
// allocations for the whole process, so a test running beside it would be
// counted too.
func TestMetricsAllocFree(t *testing.T) {
	var m metrics
	for range metricsWindow {
		m.add(16)
	}
	got := testing.AllocsPerRun(100, func() {
		m.add(16.7)
		m.percentile(0.99)
		m.budget()
	})
	if got != 0 {
		t.Errorf("measuring one frame allocated %v times, want 0", got)
	}
}

func TestDroppedFrames(t *testing.T) {
	t.Parallel()
	var m metrics
	for range metricsWindow {
		m.add(16)
	}
	before := m.dropped
	m.add(16 * 3) // three budgets: a visible stutter
	if m.dropped != before+1 {
		t.Errorf("dropped = %d, want %d after a long frame", m.dropped, before+1)
	}
	m.add(16)
	if m.dropped != before+1 {
		t.Errorf("dropped = %d, want %d: a normal frame counted as dropped", m.dropped, before+1)
	}
}

func TestFormatBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   uint64
		want string
	}{
		{0, "n/a"},
		{512, "512 B"},
		{2048, "2.0 K"},
		{3 * 1024 * 1024, "3.0 M"},
	}
	for _, tt := range tests {
		if got := formatBytes(tt.in); got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStatsReportsWhatTheBrowserCannot(t *testing.T) {
	t.Parallel()
	e := New(Config{})
	for range metricsWindow {
		e.Step(16)
	}
	s := e.Stats()

	if s.FPS < 55 || s.FPS > 65 {
		t.Errorf("FPS = %v, want about 62 from a 16 ms frame", s.FPS)
	}
	if s.Budget != 16 {
		t.Errorf("Budget = %v, want the measured 16 rather than an assumed 16.7", s.Budget)
	}
	// The two numbers no browser offers must not arrive as a plausible zero.
	if s.GPU == "" {
		t.Error("GPU is empty, want it to say why it is unknown")
	}
	if s.LongFrames != -1 {
		t.Errorf("LongFrames = %d, want -1 where the browser does not report them", s.LongFrames)
	}
}

// flat, ramp and spike build the three windows the percentile cases need.
func flat(n int, v float64) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = v
	}
	return s
}

func ramp(n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = float64(i)
	}
	return s
}

// stutter is a window with three bad frames in it. Three in a hundred and
// twenty is 2.5%, which is what a player notices and what an average hides —
// and it is the smallest share the 99th percentile can see.
func stutter(n int, base, peak float64) []float64 {
	s := flat(n, base)
	for i := range 3 {
		s[n/2+i] = peak
	}
	return s
}
