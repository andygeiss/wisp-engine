package wisp

import (
	"strconv"
	"strings"
)

// knobKind says how a value is edited and how it is spelled in Go source.
type knobKind uint8

const (
	knobFloat knobKind = iota
	knobInt
	knobBool
)

// knob is one editable value in the tuning menu.
//
// Get and Set close over a *Settings rather than reflecting on it. Reflection
// is what makes a TinyGo binary fat — and the table is short enough to keep by
// hand, which a test proves by walking Settings with reflect on the host and
// failing when a field has no knob.
type knob struct {
	Field          string
	Group          string
	Kind           knobKind
	Min, Max, Step float64
	Get            func(*Settings) float64
	Set            func(*Settings, float64)
}

// group is one nesting level of the Settings literal: the path a knob names
// and the Go type that path has.
type group struct {
	Path string
	Type string
}

// groups are the sub-structs, in the order they are declared and therefore in
// the order the literal prints them.
func groups() []group {
	return []group{
		{"Animation", "AnimationSettings"},
		{"Audio", "AudioSettings"},
		{"Camera", "CameraSettings"},
		{"Debug", "DebugSettings"},
		{"Feel", "FeelSettings"},
		{"Feel.Heavy", "ImpactSettings"},
		{"Feel.Light", "ImpactSettings"},
		{"Feel.Medium", "ImpactSettings"},
		{"Feel.Shake", "ShakeSettings"},
		{"Render", "RenderSettings"},
		{"Time", "TimeSettings"},
		{"World", "WorldSettings"},
	}
}

// boolKnob builds the two closures a bool needs, so the table stays one line
// per knob instead of five.
func boolKnob(group, field string, get func(*Settings) *bool) knob {
	return knob{
		Field: field, Group: group, Kind: knobBool, Max: 1, Step: 1,
		Get: func(s *Settings) float64 {
			if *get(s) {
				return 1
			}
			return 0
		},
		Set: func(s *Settings, v float64) { *get(s) = v != 0 },
	}
}

// intKnob does the same for a whole number.
func intKnob(group, field string, minV, maxV float64, get func(*Settings) *int) knob {
	return knob{
		Field: field, Group: group, Kind: knobInt, Min: minV, Max: maxV, Step: 1,
		Get: func(s *Settings) float64 { return float64(*get(s)) },
		Set: func(s *Settings, v float64) { *get(s) = int(v) },
	}
}

// floatKnob does the same for a number with a fraction.
func floatKnob(group, field string, minV, maxV, step float64, get func(*Settings) *float64) knob {
	return knob{
		Field: field, Group: group, Kind: knobFloat, Min: minV, Max: maxV, Step: step,
		Get: func(s *Settings) float64 { return *get(s) },
		Set: func(s *Settings, v float64) { *get(s) = v },
	}
}

// impactKnobs is the four knobs every hit strength has. Writing them once is
// what keeps Light, Medium and Heavy from drifting apart.
func impactKnobs(group string, get func(*Settings) *ImpactSettings) []knob {
	return []knob{
		floatKnob(group, "HitStopDuration", 0, 500, 5, func(s *Settings) *float64 { return &get(s).HitStopDuration }),
		floatKnob(group, "HitStopScale", 0, 1, 0.05, func(s *Settings) *float64 { return &get(s).HitStopScale }),
		floatKnob(group, "ShakeDuration", 0, 2000, 10, func(s *Settings) *float64 { return &get(s).ShakeDuration }),
		floatKnob(group, "ShakeMagnitude", 0, 32, 0.5, func(s *Settings) *float64 { return &get(s).ShakeMagnitude }),
	}
}

// knobTable returns every editable setting, grouped and in declaration order.
// It is a function rather than a package variable so the engine owns its copy
// and the package holds no mutable state.
func knobTable() []knob {
	t := []knob{
		boolKnob("Animation", "CullOffscreen", func(s *Settings) *bool { return &s.Animation.CullOffscreen }),
		intKnob("Animation", "FrameCount", 1, 32, func(s *Settings) *int { return &s.Animation.FrameCount }),
		floatKnob("Animation", "FrameDuration", 10, 500, 5, func(s *Settings) *float64 { return &s.Animation.FrameDuration }),

		floatKnob("Audio", "MusicVolume", 0, 1, 0.05, func(s *Settings) *float64 { return &s.Audio.MusicVolume }),
		floatKnob("Audio", "PitchJitter", 0, 0.5, 0.01, func(s *Settings) *float64 { return &s.Audio.PitchJitter }),
		floatKnob("Audio", "SfxVolume", 0, 1, 0.05, func(s *Settings) *float64 { return &s.Audio.SfxVolume }),
		intKnob("Audio", "Voices", 1, 16, func(s *Settings) *int { return &s.Audio.Voices }),
		floatKnob("Audio", "Volume", 0, 1, 0.05, func(s *Settings) *float64 { return &s.Audio.Volume }),

		floatKnob("Camera", "DeadzoneHeight", 0, 360, 4, func(s *Settings) *float64 { return &s.Camera.DeadzoneHeight }),
		floatKnob("Camera", "DeadzoneWidth", 0, 640, 4, func(s *Settings) *float64 { return &s.Camera.DeadzoneWidth }),
		floatKnob("Camera", "Lookahead", 0, 200, 5, func(s *Settings) *float64 { return &s.Camera.Lookahead }),
		floatKnob("Camera", "Smoothing", 0, 30, 0.5, func(s *Settings) *float64 { return &s.Camera.Smoothing }),
		boolKnob("Camera", "Snap", func(s *Settings) *bool { return &s.Camera.Snap }),

		boolKnob("Debug", "ShowHitBoxes", func(s *Settings) *bool { return &s.Debug.ShowHitBoxes }),
		boolKnob("Debug", "ShowMetrics", func(s *Settings) *bool { return &s.Debug.ShowMetrics }),
	}
	t = append(t, impactKnobs("Feel.Heavy", func(s *Settings) *ImpactSettings { return &s.Feel.Heavy })...)
	t = append(t, impactKnobs("Feel.Light", func(s *Settings) *ImpactSettings { return &s.Feel.Light })...)
	t = append(t, impactKnobs("Feel.Medium", func(s *Settings) *ImpactSettings { return &s.Feel.Medium })...)
	return append(t,
		floatKnob("Feel.Shake", "Decay", 0, 4, 0.25, func(s *Settings) *float64 { return &s.Feel.Shake.Decay }),
		floatKnob("Feel.Shake", "Frequency", 0, 120, 5, func(s *Settings) *float64 { return &s.Feel.Shake.Frequency }),
		boolKnob("Feel.Shake", "IgnoresHitStop", func(s *Settings) *bool { return &s.Feel.Shake.IgnoresHitStop }),
		boolKnob("Feel.Shake", "Snap", func(s *Settings) *bool { return &s.Feel.Shake.Snap }),

		boolKnob("Render", "PixelSnap", func(s *Settings) *bool { return &s.Render.PixelSnap }),
		boolKnob("Render", "Smoothing", func(s *Settings) *bool { return &s.Render.Smoothing }),

		floatKnob("Time", "FullscreenDebounce", 0, 2000, 50, func(s *Settings) *float64 { return &s.Time.FullscreenDebounce }),
		floatKnob("Time", "MaxStep", 5, 250, 5, func(s *Settings) *float64 { return &s.Time.MaxStep }),
		floatKnob("Time", "Scale", 0, 4, 0.05, func(s *Settings) *float64 { return &s.Time.Scale }),

		floatKnob("World", "HitBoxMargin", 0, 32, 0.5, func(s *Settings) *float64 { return &s.World.HitBoxMargin }),
		floatKnob("World", "Speed", 0, 1, 0.005, func(s *Settings) *float64 { return &s.World.Speed }),
	)
}

// clampTo keeps v inside the knob's own range, so a hand-edited saved value
// cannot brick a game.
func (k knob) clampTo(v float64) float64 {
	if v != v { // NaN, which no comparison would catch
		return k.Min
	}
	return min(max(v, k.Min), k.Max)
}

// literal spells one knob's value the way Go source does.
func (k knob) literal(s *Settings) string {
	switch k.Kind {
	case knobBool:
		if k.Get(s) != 0 {
			return "true"
		}
		return "false"
	case knobInt:
		return strconv.Itoa(int(k.Get(s)))
	}
	// 'g' with -1 digits gives the shortest text that reads back as the same
	// float64, so 0.125 stays 0.125 rather than becoming 0.12500000000000001.
	return strconv.FormatFloat(k.Get(s), 'g', -1, 64)
}

// text spells one knob's value for the menu, which has less room than Go does.
func (k knob) text(s *Settings) string {
	switch k.Kind {
	case knobBool:
		if k.Get(s) != 0 {
			return "on"
		}
		return "off"
	case knobInt:
		return strconv.Itoa(int(k.Get(s)))
	}
	return strconv.FormatFloat(k.Get(s), 'f', -1, 64)
}

// GoLiteral returns the settings as Go source: a complete wisp.Settings
// composite literal, aligned the way gofmt would align it, ready to paste into
// a game.
//
// Every field is there, so pasting it over an old one leaves nothing behind —
// a partial literal would silently zero every knob it omitted. Settings.Render
// Background is the one field it does not print, because it is a colour rather
// than a number and the menu cannot edit it.
func (s Settings) GoLiteral() string {
	var b strings.Builder
	b.WriteString("wisp.Settings{\n")

	table := knobTable()
	open := []string{} // the group path currently open, segment by segment

	for i := 0; i < len(table); {
		path := table[i].Group
		want := strings.Split(path, ".")

		// Close the levels this group has left behind, deepest first.
		for len(open) > 0 && !isPrefix(open, want) {
			open = open[:len(open)-1]
			b.WriteString(indent(len(open)+1) + "},\n")
		}
		// Open the levels it has entered.
		for len(open) < len(want) {
			open = append(open, want[len(open)])
			b.WriteString(indent(len(open)) + open[len(open)-1] + ": wisp." + typeOf(strings.Join(open, ".")) + "{\n")
		}

		// The widest field name in the group decides the padding, so the
		// pasted literal is already gofmt-clean.
		w, n := 0, 0
		for ; i+n < len(table) && table[i+n].Group == path; n++ {
			w = max(w, len(table[i+n].Field))
		}
		for _, k := range table[i : i+n] {
			b.WriteString(indent(len(open)+1) + k.Field + ":")
			b.WriteString(strings.Repeat(" ", w-len(k.Field)+1))
			b.WriteString(k.literal(&s))
			b.WriteString(",\n")
		}
		i += n
	}

	for len(open) > 0 {
		open = open[:len(open)-1]
		b.WriteString(indent(len(open)+1) + "},\n")
	}
	b.WriteString("}")
	return b.String()
}

// MarshalText writes the settings as one name=value line per knob. It is what
// the browser remembers between reloads.
//
// It is not JSON: encoding/json needs reflection, and reflection is what makes
// a TinyGo binary large enough to notice.
func (s Settings) MarshalText() ([]byte, error) {
	var b strings.Builder
	for _, k := range knobTable() {
		b.WriteString(k.Group)
		b.WriteByte('.')
		b.WriteString(k.Field)
		b.WriteByte('=')
		b.WriteString(k.literal(&s))
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

// UnmarshalText reads what MarshalText wrote, over the top of whatever the
// settings already hold.
//
// It never fails. A name it does not know is skipped, a name that is missing
// keeps its current value, and every value is clamped to its own range — so an
// old save, a hand-edited one, or a Time.Scale of 1e308 typed into the
// browser's console cannot leave a game unplayable.
func (s *Settings) UnmarshalText(text []byte) error {
	table := knobTable()
	for line := range strings.Lines(string(text)) {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			// A bool is written as true or false, not as a number.
			switch strings.TrimSpace(value) {
			case "true":
				v = 1
			case "false":
				v = 0
			default:
				continue
			}
		}
		for _, k := range table {
			if k.Group+"."+k.Field == name {
				k.Set(s, k.clampTo(v))
				break
			}
		}
	}
	return nil
}

// indent returns n tabs, so the generated literal is tab-indented like the
// rest of Go.
func indent(n int) string { return strings.Repeat("\t", n) }

// isPrefix reports whether have is a prefix of want.
func isPrefix(have, want []string) bool {
	if len(have) > len(want) {
		return false
	}
	for i, s := range have {
		if want[i] != s {
			return false
		}
	}
	return true
}

// typeOf returns the Go type name of a group path.
func typeOf(path string) string {
	for _, g := range groups() {
		if g.Path == path {
			return g.Type
		}
	}
	return ""
}

// trimFloat spells a number for the menu footer, without a trailing zero.
func trimFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
