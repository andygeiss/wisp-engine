package wisp_test

import (
	"go/parser"
	"reflect"
	"strings"
	"testing"

	"github.com/andygeiss/wisp-engine"
)

// TestGoLiteralParses is the promise the menu's copy key makes: what lands on
// the clipboard is Go a person can paste. Parsing it is how that is checked
// without writing it to a file and compiling it.
func TestGoLiteralParses(t *testing.T) {
	t.Parallel()
	s := wisp.Defaults()
	s.Feel.Heavy.ShakeMagnitude = 12.5
	s.World.Speed = 0.125
	s.Render.Smoothing = true

	src := s.GoLiteral()

	if _, err := parser.ParseExpr(src); err != nil {
		t.Fatalf("the literal does not parse: %v\n\n%s", err, src)
	}
	// Assert on what the literal says, not on how gofmt spaces it: the
	// padding is an implementation detail and pinning it makes this brittle.
	for _, want := range []string{
		"wisp.Settings{",
		"Feel: wisp.FeelSettings{",
		"Heavy: wisp.ImpactSettings{",
		"ShakeMagnitude:",
		"12.5,",
		"0.125,",
		"true,",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the literal is missing %q:\n\n%s", want, src)
		}
	}

	// Every knob must be there: a partial literal pasted over a tuned one
	// would silently reset whatever it left out.
	if got, want := strings.Count(src, ":"), len(knobNames(t)); got < want {
		t.Errorf("the literal has %d fields, want at least %d", got, want)
	}
}

// TestKnobTableCoversSettings is the safety net under the hand-written table.
//
// The table is hand-written because reflection costs a TinyGo binary real
// bytes. This test runs under ordinary Go on the host, where reflection is
// free, so the check costs the shipped module nothing: it walks every leaf
// field of Settings and fails when one has no knob. Add a field, forget the
// knob, and the menu would silently never show it — this is what catches that.
func TestKnobTableCoversSettings(t *testing.T) {
	t.Parallel()

	// Background is a colour, not a number, so no menu row can edit it.
	skip := map[string]bool{"Render.Background": true}

	var leaves []string
	var walk func(t reflect.Type, path string)
	walk = func(rt reflect.Type, path string) {
		for f := range rt.Fields() {
			name := f.Name
			if path != "" {
				name = path + "." + f.Name
			}
			if f.Type.Kind() == reflect.Struct {
				walk(f.Type, name)
				continue
			}
			leaves = append(leaves, name)
		}
	}
	walk(reflect.TypeFor[wisp.Settings](), "")

	// A knob's name shows up in the text format, which is the one place the
	// table's names are visible from outside the package.
	text, err := wisp.Defaults().MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	named := map[string]bool{}
	for line := range strings.Lines(string(text)) {
		if name, _, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			named[name] = true
		}
	}

	for _, leaf := range leaves {
		if skip[leaf] || named[leaf] {
			continue
		}
		t.Errorf("Settings.%s has no knob, so the tuning menu cannot reach it", leaf)
	}
	if len(named) != len(leaves)-len(skip) {
		t.Errorf("the table has %d knobs for %d settings fields", len(named), len(leaves)-len(skip))
	}
}

// TestDefaultsAreInRange catches a typo'd range before the menu does: a
// default outside its own minimum and maximum would jump the moment somebody
// nudged it.
func TestDefaultsAreInRange(t *testing.T) {
	t.Parallel()
	def := wisp.Defaults()

	// Round-tripping through the text format clamps every value to its range,
	// so anything that moves was outside it.
	text, err := def.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	var got wisp.Settings
	if err := got.UnmarshalText(text); err != nil {
		t.Fatalf("UnmarshalText: %v", err)
	}
	got.Render.Background = def.Render.Background // not a knob

	if got != def {
		t.Errorf("a default was clamped, so it sits outside its own range:\ngot  %+v\nwant %+v", got, def)
	}
}

func TestUnmarshalText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		text  string
		check func(*testing.T, wisp.Settings)
	}{
		{"a value is read", "World.Speed=0.5\n", func(t *testing.T, s wisp.Settings) {
			if s.World.Speed != 0.5 {
				t.Errorf("Speed = %v, want 0.5", s.World.Speed)
			}
		}},
		{"a bool is read", "Render.Smoothing=true\n", func(t *testing.T, s wisp.Settings) {
			if !s.Render.Smoothing {
				t.Error("Smoothing = false, want true")
			}
		}},
		{"an unknown name is skipped", "Nope.Nothing=1\nWorld.Speed=0.25\n", func(t *testing.T, s wisp.Settings) {
			if s.World.Speed != 0.25 {
				t.Errorf("Speed = %v, want 0.25: an unknown name ate the next line", s.World.Speed)
			}
		}},
		{"a missing name keeps its default", "", func(t *testing.T, s wisp.Settings) {
			if s.World.Speed != wisp.Defaults().World.Speed {
				t.Errorf("Speed = %v, want the default", s.World.Speed)
			}
		}},
		{"a value past the maximum is clamped", "Time.Scale=1e308\n", func(t *testing.T, s wisp.Settings) {
			if s.Time.Scale != 4 {
				t.Errorf("Scale = %v, want it clamped to 4", s.Time.Scale)
			}
		}},
		{"a value below the minimum is clamped", "World.Speed=-99\n", func(t *testing.T, s wisp.Settings) {
			if s.World.Speed != 0 {
				t.Errorf("Speed = %v, want it clamped to 0", s.World.Speed)
			}
		}},
		{"NaN falls back to the minimum", "Time.MaxStep=NaN\n", func(t *testing.T, s wisp.Settings) {
			if s.Time.MaxStep != 5 {
				t.Errorf("MaxStep = %v, want the minimum of 5", s.Time.MaxStep)
			}
		}},
		{"a line with no = is skipped", "garbage\nWorld.Speed=0.75\n", func(t *testing.T, s wisp.Settings) {
			if s.World.Speed != 0.75 {
				t.Errorf("Speed = %v, want 0.75", s.World.Speed)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := wisp.Defaults()
			if err := s.UnmarshalText([]byte(tt.text)); err != nil {
				t.Fatalf("UnmarshalText: %v", err)
			}
			tt.check(t, s)
		})
	}
}

// FuzzUnmarshalText guards the one place untrusted bytes reach the settings:
// the browser's storage, which anybody can edit from a console. The contract is
// that it never panics and never leaves a knob outside its own range.
func FuzzUnmarshalText(f *testing.F) {
	text, err := wisp.Defaults().MarshalText()
	if err != nil {
		f.Fatalf("MarshalText: %v", err)
	}
	f.Add(text)
	f.Add([]byte(""))
	f.Add([]byte("="))
	f.Add([]byte("World.Speed=NaN\n"))
	f.Add([]byte("Time.Scale=1e400\n"))
	f.Add([]byte("Audio.Voices=-2147483648\n"))
	f.Add([]byte(strings.Repeat("A", 10000)))

	f.Fuzz(func(t *testing.T, b []byte) {
		s := wisp.Defaults()
		if err := s.UnmarshalText(b); err != nil {
			t.Fatalf("UnmarshalText returned an error, which it never should: %v", err)
		}
		// Marshalling and reading back must not move anything: if it does, a
		// value escaped its range on the way in.
		once, err := s.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText: %v", err)
		}
		again := wisp.Defaults()
		if err := again.UnmarshalText(once); err != nil {
			t.Fatalf("UnmarshalText: %v", err)
		}
		twice, err := again.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText: %v", err)
		}
		if string(once) != string(twice) {
			t.Errorf("a value moved on the second pass:\n%s\nbecame\n%s", once, twice)
		}
	})
}

// knobNames returns the name of every knob, read back out of the text format
// because that is the one place the table is visible from outside the package.
func knobNames(t *testing.T) []string {
	t.Helper()
	text, err := wisp.Defaults().MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	var names []string
	for line := range strings.Lines(string(text)) {
		if name, _, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			names = append(names, name)
		}
	}
	return names
}
