package wisp_test

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/andygeiss/wisp-engine"
)

// exportPath is a real Aseprite export — the lab's old sheet, written by
// Aseprite 1.3 with the command ParseSheet documents — kept as the parser's
// fixture now that the lab draws PixelLab's art. The tests read a real file
// rather than a hand-written copy of one: a fixture somebody typed only
// proves the parser agrees with a guess about the format, and the guess is
// the part most likely to be wrong.
const exportPath = "testdata/aseprite/lab.json"

// readExport returns the committed Aseprite export.
func readExport(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("reading %s: %v", exportPath, err)
	}
	return data
}

func TestParseSheetReadsTheExport(t *testing.T) {
	t.Parallel()

	sheet, err := wisp.ParseSheet(readExport(t))
	if err != nil {
		t.Fatalf("ParseSheet: %v", err)
	}

	if got, want := len(sheet.Frames), 96; got != want {
		t.Errorf("frames = %d, want %d", got, want)
	}
	if sheet.W != 256 || sheet.H != 384 {
		t.Errorf("image = %vx%v, want 256x384", sheet.W, sheet.H)
	}

	// Frame 8 is the first of the second row, which is what says the export
	// really is laid out as the grid and not as one long strip.
	f := sheet.Frames[8]
	if f.X != 0 || f.Y != 32 || f.W != 32 || f.H != 32 {
		t.Errorf("frame 8 = %+v, want 0,32 32x32", f)
	}
	if f.Duration != 100 {
		t.Errorf("frame 8 duration = %v, want 100", f.Duration)
	}
}

func TestParseSheetReadsTheTags(t *testing.T) {
	t.Parallel()

	sheet, err := wisp.ParseSheet(readExport(t))
	if err != nil {
		t.Fatalf("ParseSheet: %v", err)
	}
	if got, want := len(sheet.Tags), 12; got != want {
		t.Fatalf("tags = %d, want %d", got, want)
	}

	// The first four are the ones cmd/lab draws with, and their indices are
	// the rows its RowForState already names.
	want := []struct {
		name     string
		from, to int
	}{
		{"idle-right", 0, 7},
		{"idle-left", 8, 15},
		{"run-right", 16, 23},
		{"run-left", 24, 31},
	}
	for i, w := range want {
		got := sheet.Tags[i]
		if got.Name != w.name || got.From != w.from || got.To != w.to {
			t.Errorf("tag %d = %q %d..%d, want %q %d..%d",
				i, got.Name, got.From, got.To, w.name, w.from, w.to)
		}
		if got.Direction != wisp.Forward {
			t.Errorf("tag %d direction = %v, want Forward", i, got.Direction)
		}
	}

	if i := sheet.Lookup("run-right"); i != 2 {
		t.Errorf("Lookup(run-right) = %d, want 2", i)
	}
	if i := sheet.Lookup("no-such-animation"); i != -1 {
		t.Errorf("Lookup(missing) = %d, want -1", i)
	}
}

// TestGridSheetMatchesTheExport is the backwards-compatibility proof: the grid
// convention and the Aseprite export describe the same pixels, frame for
// frame. It is what says a game may move to a sheet without its sprites
// moving, and it is why the fixture was exported at the grid width rather
// than packed.
func TestGridSheetMatchesTheExport(t *testing.T) {
	t.Parallel()

	parsed, err := wisp.ParseSheet(readExport(t))
	if err != nil {
		t.Fatalf("ParseSheet: %v", err)
	}
	grid := wisp.GridSheet(8, 12, 32, 32, 100)

	if grid.W != parsed.W || grid.H != parsed.H {
		t.Errorf("image = %vx%v, want %vx%v", grid.W, grid.H, parsed.W, parsed.H)
	}
	if len(grid.Frames) != len(parsed.Frames) {
		t.Fatalf("frames = %d, want %d", len(grid.Frames), len(parsed.Frames))
	}
	for i := range grid.Frames {
		if grid.Frames[i] != parsed.Frames[i] {
			t.Errorf("frame %d = %+v, want %+v", i, grid.Frames[i], parsed.Frames[i])
		}
	}

	// The tags line up too, so RowForState keeps pointing at the animation it
	// always pointed at. Only the names are new, which is the whole of what
	// the export adds.
	if len(grid.Tags) != len(parsed.Tags) {
		t.Fatalf("tags = %d, want %d", len(grid.Tags), len(parsed.Tags))
	}
	for i := range grid.Tags {
		g, p := grid.Tags[i], parsed.Tags[i]
		if g.From != p.From || g.To != p.To || g.Direction != p.Direction {
			t.Errorf("tag %d = %d..%d dir %v, want %d..%d dir %v",
				i, g.From, g.To, g.Direction, p.From, p.To, p.Direction)
		}
		if g.Name != "" {
			t.Errorf("grid tag %d is named %q; a grid says nothing about names", i, g.Name)
		}
	}
}

func TestGridSheetRefusesAnEmptyGrid(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ cols, rows int }{{0, 4}, {4, 0}, {-1, 4}, {4, -1}} {
		if s := wisp.GridSheet(tc.cols, tc.rows, 32, 32, 100); len(s.Frames) != 0 {
			t.Errorf("GridSheet(%d, %d) made %d frames, want none",
				tc.cols, tc.rows, len(s.Frames))
		}
	}
}

func TestParseSheetRejects(t *testing.T) {
	t.Parallel()

	const size = `"meta":{"size":{"w":64,"h":64}}`
	frame := func(body string) string {
		return `{"frames":[{"frame":` + body + `,"duration":100}],` + size + `}`
	}

	tests := []struct {
		name string
		in   string
		want error
	}{
		{"empty", ``, wisp.ErrSheetSyntax},
		{"not an object", `[]`, wisp.ErrSheetSyntax},
		{"truncated", `{"frames":[`, wisp.ErrSheetSyntax},
		{"trailing rubble", frame(`{"x":0,"y":0,"w":8,"h":8}`) + `x`, wisp.ErrSheetSyntax},
		{"no meta", `{"frames":[{"frame":{"x":0,"y":0,"w":8,"h":8}}]}`, wisp.ErrSheetSyntax},
		{"no frames", `{"frames":[],` + size + `}`, wisp.ErrSheetSyntax},
		{"bad escape", `{"frames":[{"a":"\q"}],` + size + `}`, wisp.ErrSheetSyntax},
		{"unterminated string", `{"frames":[{"a":"x}],` + size + `}`, wisp.ErrSheetSyntax},
		{"number is not one", frame(`{"x":1.2.3,"y":0,"w":8,"h":8}`), wisp.ErrSheetSyntax},

		{"infinite coordinate", frame(`{"x":1e400,"y":0,"w":8,"h":8}`), wisp.ErrSheetRange},
		{"zero width", frame(`{"x":0,"y":0,"w":0,"h":8}`), wisp.ErrSheetRange},
		{"negative origin", frame(`{"x":-1,"y":0,"w":8,"h":8}`), wisp.ErrSheetRange},
		{"rect leaves the image", frame(`{"x":60,"y":0,"w":8,"h":8}`), wisp.ErrSheetRange},
		{"image has no size", `{"frames":[{"frame":{"x":0,"y":0,"w":8,"h":8}}],"meta":{"size":{"w":0,"h":0}}}`, wisp.ErrSheetRange},
		{"tag past the frames", `{"frames":[{"frame":{"x":0,"y":0,"w":8,"h":8}}],` +
			`"meta":{"size":{"w":64,"h":64},"frameTags":[{"name":"a","from":0,"to":9}]}}`, wisp.ErrSheetRange},
		{"tag runs backwards", `{"frames":[{"frame":{"x":0,"y":0,"w":8,"h":8}}],` +
			`"meta":{"size":{"w":64,"h":64},"frameTags":[{"name":"a","from":1,"to":0}]}}`, wisp.ErrSheetRange},
		{"fractional tag index", `{"frames":[{"frame":{"x":0,"y":0,"w":8,"h":8}}],` +
			`"meta":{"size":{"w":64,"h":64},"frameTags":[{"name":"a","from":0.5,"to":0.5}]}}`, wisp.ErrSheetRange},

		// The nesting has to be under a key the scanner skips: the frames
		// array refuses a "[" where it wants a frame object long before depth
		// could run out, so a deep file only reaches the limit on the path
		// that steps over what it does not know.
		{"deep array", `{"other":` + strings.Repeat("[", 200), wisp.ErrSheetTooDeep},
		{"deep object", `{"other":` + strings.Repeat(`{"k":`, 200), wisp.ErrSheetTooDeep},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := wisp.ParseSheet([]byte(tc.in)); !errors.Is(err, tc.want) {
				t.Errorf("ParseSheet(%q) = %v, want %v", tc.in, err, tc.want)
			}
		})
	}
}

func TestParseSheetRefusesAHugeInput(t *testing.T) {
	t.Parallel()

	huge := make([]byte, (4<<20)+1)
	if _, err := wisp.ParseSheet(huge); !errors.Is(err, wisp.ErrSheetTooLarge) {
		t.Errorf("ParseSheet(4 MiB + 1) = %v, want ErrSheetTooLarge", err)
	}
}

func TestParseSheetClampsDurations(t *testing.T) {
	t.Parallel()

	const in = `{"frames":[
		{"frame":{"x":0,"y":0,"w":8,"h":8},"duration":0},
		{"frame":{"x":8,"y":0,"w":8,"h":8},"duration":999999}
	],"meta":{"size":{"w":64,"h":64}}}`

	sheet, err := wisp.ParseSheet([]byte(in))
	if err != nil {
		t.Fatalf("ParseSheet: %v", err)
	}
	if got := sheet.Frames[0].Duration; got != 1 {
		t.Errorf("duration 0 became %v, want 1", got)
	}
	if got := sheet.Frames[1].Duration; got != 60000 {
		t.Errorf("duration 999999 became %v, want 60000", got)
	}
}

// TestParseSheetSkipsWhatItDoesNotKnow is the promise that a newer Aseprite is
// not a breaking change: every field the scanner has no use for is stepped
// over without being interpreted.
func TestParseSheetSkipsWhatItDoesNotKnow(t *testing.T) {
	t.Parallel()

	const in = `{
		"future": {"nested": [1, 2, {"deep": null}], "flag": true},
		"frames":[{"filename":"a é 😀 b","frame":{"x":0,"y":0,"w":8,"h":8},
		           "rotated":false,"trimmed":false,"duration":50,"unknown":[[[]]]}],
		"meta":{"app":"x","size":{"w":64,"h":64},"scale":"1",
		        "frameTags":[{"name":"a","from":0,"to":0,"direction":"pingpong","color":"#000000ff"}],
		        "slices":[]}
	}`
	sheet, err := wisp.ParseSheet([]byte(in))
	if err != nil {
		t.Fatalf("ParseSheet: %v", err)
	}
	if len(sheet.Frames) != 1 || sheet.Frames[0].Duration != 50 {
		t.Errorf("frames = %+v, want one frame of 50 ms", sheet.Frames)
	}
	if len(sheet.Tags) != 1 || sheet.Tags[0].Direction != wisp.PingPong {
		t.Errorf("tags = %+v, want one pingpong tag", sheet.Tags)
	}
}

func TestParseSheetReadsDirections(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		spelled string
		want    wisp.Direction
	}{
		{"forward", wisp.Forward},
		{"reverse", wisp.Reverse},
		{"pingpong", wisp.PingPong},
		{"a-mode-aseprite-adds-later", wisp.Forward},
	} {
		in := `{"frames":[{"frame":{"x":0,"y":0,"w":8,"h":8},"duration":100}],` +
			`"meta":{"size":{"w":64,"h":64},"frameTags":[` +
			`{"name":"a","from":0,"to":0,"direction":"` + tc.spelled + `"}]}}`
		sheet, err := wisp.ParseSheet([]byte(in))
		if err != nil {
			t.Fatalf("ParseSheet(%s): %v", tc.spelled, err)
		}
		if got := sheet.Tags[0].Direction; got != tc.want {
			t.Errorf("direction %q = %v, want %v", tc.spelled, got, tc.want)
		}
	}
}

func TestPlayStartsTheNamedAnimation(t *testing.T) {
	e := newEngine(t)
	sheet, err := wisp.ParseSheet(readExport(t))
	if err != nil {
		t.Fatalf("ParseSheet: %v", err)
	}
	e.Sheets = []wisp.Sheet{sheet}
	i := e.Add(wisp.Sprite{Height: 32, Width: 32, State: wisp.StateVisible})

	e.FrameOffset[i], e.FrameTime[i] = 5, 40
	if !e.Play(i, "run-left") {
		t.Fatal("Play(run-left) = false, want true")
	}
	if got := e.ImageRow[i]; got != 3 {
		t.Errorf("ImageRow = %d, want 3", got)
	}
	if e.FrameOffset[i] != 0 || e.FrameTime[i] != 0 {
		t.Errorf("frame = %d at %v ms, want 0 at 0 — Play restarts the animation",
			e.FrameOffset[i], e.FrameTime[i])
	}
	if got := e.Playing(i); got != "run-left" {
		t.Errorf("Playing = %q, want run-left", got)
	}
}

func TestPlayReportsWhatItCouldNotFind(t *testing.T) {
	e := newEngine(t)
	i := e.Add(wisp.Sprite{Height: 32, Width: 32})

	// No sheet at all: the grid convention is in charge and there is nothing
	// to look a name up in.
	if e.Play(i, "run-left") {
		t.Error("Play with no sheet = true, want false")
	}
	if got := e.Playing(i); got != "" {
		t.Errorf("Playing with no sheet = %q, want empty", got)
	}

	sheet, err := wisp.ParseSheet(readExport(t))
	if err != nil {
		t.Fatalf("ParseSheet: %v", err)
	}
	e.Sheets = []wisp.Sheet{sheet}
	if e.Play(i, "no-such-animation") {
		t.Error("Play with an unknown name = true, want false")
	}
	if got := e.ImageRow[i]; got != 0 {
		t.Errorf("ImageRow = %d after a failed Play, want it left alone", got)
	}
}

// TestSheetTimesFramesItself is the difference a sheet makes to the update: the
// frame duration comes from the sheet, not from AnimationSettings.
func TestSheetTimesFramesItself(t *testing.T) {
	e := newEngine(t)
	e.Animation.FrameDuration = 1000 // Deliberately nothing like the sheet's.
	e.Sheets = []wisp.Sheet{wisp.GridSheet(4, 1, 8, 8, 25)}
	i := e.Add(wisp.Sprite{
		Height: 8, Width: 8,
		State: wisp.StateAnimated | wisp.StateAnimatedLoop | wisp.StateVisible,
	})

	e.Step(25)
	if got := e.FrameOffset[i]; got != 1 {
		t.Errorf("frame = %d after 25 ms, want 1 — the sheet says 25 ms a frame", got)
	}
}

func TestSheetEndsAOneShotAfterItsOwnFrames(t *testing.T) {
	e := newEngine(t)
	e.Animation.FrameCount = 8 // Again, nothing like the sheet's three.
	e.Sheets = []wisp.Sheet{wisp.GridSheet(3, 1, 8, 8, 10)}
	i := e.Add(wisp.Sprite{
		Height: 8, Width: 8,
		State: wisp.StateAnimated | wisp.StateAutoHide | wisp.StateVisible,
	})

	for range 3 {
		e.Step(10)
	}
	if e.State[i]&wisp.StateAnimated != 0 {
		t.Error("still animating after three frames of a three-frame tag")
	}
	if e.State[i]&wisp.StateVisible != 0 {
		t.Error("StateAutoHide did not hide the entity when the animation ended")
	}
}

func ExampleParseSheet() {
	// In a game this is the file Aseprite exported, fetched next to the PNG.
	const export = `{
	  "frames": [
	    {"frame": {"x": 0, "y": 0, "w": 32, "h": 32}, "duration": 100},
	    {"frame": {"x": 32, "y": 0, "w": 32, "h": 32}, "duration": 100}
	  ],
	  "meta": {
	    "size": {"w": 64, "h": 32},
	    "frameTags": [{"name": "walk", "from": 0, "to": 1, "direction": "forward"}]
	  }
	}`

	sheet, err := wisp.ParseSheet([]byte(export))
	if err != nil {
		panic(err)
	}

	e := wisp.New(wisp.Config{})
	e.Sheets = []wisp.Sheet{sheet}
	hero := e.Add(wisp.Sprite{
		Height: 32, Width: 32,
		State: wisp.StateAnimated | wisp.StateAnimatedLoop | wisp.StateVisible,
	})
	e.Play(hero, "walk")

	fmt.Println(len(sheet.Frames), sheet.Tags[0].Name, e.Playing(hero))
	// Output: 2 walk walk
}

// FuzzParseSheet feeds the scanner bytes nobody wrote on purpose. The parser
// reads a file produced by another program, so the interesting question is not
// whether it rejects rubbish but whether anything it *accepts* is safe to
// draw: every assertion below is one the engine relies on and never rechecks.
func FuzzParseSheet(f *testing.F) {
	f.Add(readExport(&testing.T{}))
	f.Add([]byte(""))
	f.Add([]byte("{}"))
	f.Add([]byte(`{"frames":[]}`))
	f.Add([]byte(`{"frames":[{"frame":{"x":0,"y":0,"w":8,"h":8},"duration":100}],"meta":{"size":{"w":8,"h":8}}}`))
	f.Add([]byte(`{"frames":[{"frame":{"x":0,"y":0,"w":1e400,"h":8}}],"meta":{"size":{"w":8,"h":8}}}`))
	f.Add([]byte(`{"meta":{"frameTags":[{"name":"😀","from":0,"to":0}]}}`))
	f.Add([]byte(`{"meta":{"frameTags":[{"name":"\ud800","from":-1,"to":99999999999}]}}`))
	f.Add([]byte(`{"other":` + strings.Repeat("[", 500)))
	f.Add([]byte(strings.Repeat(`{"a":`, 100)))

	f.Fuzz(func(t *testing.T, data []byte) {
		sheet, err := wisp.ParseSheet(data)
		if err != nil {
			// Every rejection is one of the documented five, so a caller can
			// always branch on what went wrong.
			switch {
			case errors.Is(err, wisp.ErrSheetTooLarge),
				errors.Is(err, wisp.ErrSheetSyntax),
				errors.Is(err, wisp.ErrSheetTooDeep),
				errors.Is(err, wisp.ErrSheetTooMany),
				errors.Is(err, wisp.ErrSheetRange):
			default:
				t.Fatalf("ParseSheet returned an unsentinelled error: %v", err)
			}
			return
		}

		if len(sheet.Frames) == 0 {
			t.Fatal("accepted a sheet with no frames")
		}
		if len(sheet.Frames) > 65535 || len(sheet.Tags) > 1024 {
			t.Fatalf("accepted %d frames and %d tags, over the limits",
				len(sheet.Frames), len(sheet.Tags))
		}
		for i, fr := range sheet.Frames {
			if fr.W <= 0 || fr.H <= 0 || fr.X < 0 || fr.Y < 0 {
				t.Fatalf("frame %d has no area: %+v", i, fr)
			}
			if fr.X+fr.W > sheet.W || fr.Y+fr.H > sheet.H {
				t.Fatalf("frame %d leaves the %vx%v image: %+v", i, sheet.W, sheet.H, fr)
			}
			if fr.Duration < 1 || fr.Duration > 60000 {
				t.Fatalf("frame %d lasts %v ms, outside the clamp", i, fr.Duration)
			}
		}
		for i, tg := range sheet.Tags {
			if tg.From < 0 || tg.To < tg.From || tg.To >= len(sheet.Frames) {
				t.Fatalf("tag %d names frames %d..%d of %d",
					i, tg.From, tg.To, len(sheet.Frames))
			}
		}

		// Anything the sheet accepted has to be drawable: no offset of any tag
		// may address a frame that is not there. This is the invariant the
		// draw loop trusts without checking, every frame, for every entity.
		for _, tg := range sheet.Tags {
			if !utf8.ValidString(tg.Name) {
				t.Fatalf("tag name is not valid UTF-8: %q", tg.Name)
			}
		}
	})
}

// TestParseSheetUndoesEscapes covers the half of a tag name nobody writes on
// purpose. A name is text a person typed in Aseprite, so it can carry anything
// JSON can spell — and a surrogate pair is spelled as two escapes, which is
// where the reader has to step onto the second one rather than past it.
func TestParseSheetUndoesEscapes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, in, want string
	}{
		{"the two-character ones", `a\tb\nc\\d\/e\"f`, "a\tb\nc\\d/e\"f"},
		{"a basic-plane escape", `\u00e9`, "\u00e9"},
		{"a surrogate pair", `\ud83d\ude00`, "\U0001F600"},
		{"a pair with text after it", `\ud83d\ude00walk`, "\U0001F600walk"},
		{"a lone high half", `\ud83dx`, "\uFFFDx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := `{"frames":[{"frame":{"x":0,"y":0,"w":8,"h":8},"duration":9}],` +
				`"meta":{"size":{"w":8,"h":8},"frameTags":[{"name":"` + tc.in +
				`","from":0,"to":0}]}}`
			sheet, err := wisp.ParseSheet([]byte(in))
			if err != nil {
				t.Fatalf("ParseSheet: %v", err)
			}
			if got := sheet.Tags[0].Name; got != tc.want {
				t.Errorf("%s became %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
