package wisp

import (
	"errors"
	"math"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// The parser is written by hand rather than handed to encoding/json, and that
// is a TinyGo decision rather than a taste one. encoding/json reads a struct
// with reflection, which TinyGo supports only partly: the cost is tens of
// kilobytes of a module that has to stay under a size gate, and the failure
// mode is a reflect panic in a browser on a build every gate called green.
//
// So this reads the six things the engine needs — the frame rectangles, the
// durations, the image size and the tags — and steps over every other value
// without interpreting it. That is also what lets an Aseprite release add a
// field without breaking anything here.

// The limits. Every one of them is checked, because a parser reading a file
// somebody else produced is the place a game meets bytes it did not write.
const (
	// maxSheetBytes is the largest export accepted. A sheet describing tens of
	// thousands of frames is well under this.
	maxSheetBytes = 4 << 20
	// maxSheetDepth caps nesting, so a file made of ten thousand open brackets
	// cannot run the parser out of stack.
	maxSheetDepth = 64
	// maxSheetFrames and maxSheetTags cap what one sheet may describe.
	maxSheetFrames = 65535
	maxSheetTags   = 1024
	// The bounds a frame's duration is clamped into: never zero, because a
	// zero-length frame is an animation that never advances, and never longer
	// than a minute.
	minFrameMS = 1
	maxFrameMS = 60000
)

// The failures a caller can branch on. Everything the parser rejects comes
// back wrapped around one of these, so a game can tell "this is not the file I
// meant" from "this file is too big to be one".
var (
	// ErrSheetTooLarge means the input is over maxSheetBytes.
	ErrSheetTooLarge = errors.New("wisp: sheet input too large")
	// ErrSheetSyntax means the bytes are not JSON, or are JSON that is not an
	// Aseprite json-array export.
	ErrSheetSyntax = errors.New("wisp: malformed sheet")
	// ErrSheetTooDeep means the nesting is over maxSheetDepth.
	ErrSheetTooDeep = errors.New("wisp: sheet nested too deeply")
	// ErrSheetTooMany means there are more frames or tags than a sheet may
	// hold.
	ErrSheetTooMany = errors.New("wisp: too many frames or tags in sheet")
	// ErrSheetRange means a number is not finite, a frame's rectangle leaves
	// the image, or a tag names frames that do not exist.
	ErrSheetRange = errors.New("wisp: sheet value out of range")
)

// ParseSheet reads an Aseprite sprite sheet export and returns the [Sheet] it
// describes.
//
// Produce the input with Aseprite's own exporter, which is what "make sheets"
// runs:
//
//	aseprite -b assets/lab.aseprite --sheet lab.png --sheet-type rows \
//	  --data lab.json --format json-array --list-tags
//
// The format must be json-array and the tags must be listed; without
// --list-tags an export carries no animations and every frame is one nameless
// run. Fields ParseSheet does not know are skipped, so a newer Aseprite is not
// a breaking change.
//
// The returned Sheet is checked, not merely parsed: every frame's rectangle is
// inside the image, every tag names frames that exist, and every duration is a
// real number of milliseconds. A game may attach the result to
// [Engine.Sheets] without looking at it.
func ParseSheet(data []byte) (Sheet, error) {
	if len(data) > maxSheetBytes {
		return Sheet{}, ErrSheetTooLarge
	}
	sc := &scanner{s: data}
	var sh Sheet
	sized := false

	err := sc.object(func(key string) error {
		switch key {
		case "frames":
			return sc.frames(&sh)
		case "meta":
			return sc.meta(&sh, &sized)
		}
		return sc.skip()
	})
	if err != nil {
		return Sheet{}, err
	}
	if err := sc.atEnd(); err != nil {
		return Sheet{}, err
	}
	if !sized || len(sh.Frames) == 0 {
		return Sheet{}, ErrSheetSyntax
	}
	if err := sh.check(); err != nil {
		return Sheet{}, err
	}
	return sh, nil
}

// check is the pass that runs once everything is read. It cannot run earlier:
// Aseprite writes meta after frames, so the image size a rectangle is checked
// against is not known while the rectangles are being read.
func (s *Sheet) check() error {
	if !finite(s.W) || !finite(s.H) || s.W <= 0 || s.H <= 0 {
		return ErrSheetRange
	}
	for i := range s.Frames {
		f := &s.Frames[i]
		if !finite(f.X) || !finite(f.Y) || !finite(f.W) || !finite(f.H) {
			return ErrSheetRange
		}
		if f.W <= 0 || f.H <= 0 || f.X < 0 || f.Y < 0 {
			return ErrSheetRange
		}
		if f.X+f.W > s.W || f.Y+f.H > s.H {
			return ErrSheetRange
		}
		// A duration is clamped rather than refused. Aseprite writes 0 for a
		// frame the artist set to zero, and that is a sheet worth drawing with
		// one frame a millisecond, not a file worth rejecting.
		if !finite(f.Duration) {
			return ErrSheetRange
		}
		f.Duration = min(max(f.Duration, minFrameMS), maxFrameMS)
	}
	for _, t := range s.Tags {
		if t.From < 0 || t.To < t.From || t.To >= len(s.Frames) {
			return ErrSheetRange
		}
	}
	return nil
}

// finite reports whether f is a real number. JSON cannot spell NaN or an
// infinity, but 1e400 parses to one, so the check is on the value rather than
// on the text.
func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

// frames reads the "frames" array: a rectangle and a duration each, and
// nothing else the export puts there.
func (sc *scanner) frames(sh *Sheet) error {
	return sc.array(func() error {
		if len(sh.Frames) >= maxSheetFrames {
			return ErrSheetTooMany
		}
		var f Frame
		err := sc.object(func(key string) error {
			switch key {
			case "frame":
				return sc.rect(&f)
			case "duration":
				return sc.number(&f.Duration)
			}
			return sc.skip()
		})
		if err != nil {
			return err
		}
		sh.Frames = append(sh.Frames, f)
		return nil
	})
}

// rect reads one {x, y, w, h} object into f.
func (sc *scanner) rect(f *Frame) error {
	return sc.object(func(key string) error {
		switch key {
		case "x":
			return sc.number(&f.X)
		case "y":
			return sc.number(&f.Y)
		case "w":
			return sc.number(&f.W)
		case "h":
			return sc.number(&f.H)
		}
		return sc.skip()
	})
}

// meta reads the image size and the tags. sized says whether a size was found,
// because a zero size and a missing one are different mistakes.
func (sc *scanner) meta(sh *Sheet, sized *bool) error {
	return sc.object(func(key string) error {
		switch key {
		case "size":
			*sized = true
			return sc.object(func(k string) error {
				switch k {
				case "w":
					return sc.number(&sh.W)
				case "h":
					return sc.number(&sh.H)
				}
				return sc.skip()
			})
		case "frameTags":
			return sc.tags(sh)
		}
		return sc.skip()
	})
}

// tags reads the "frameTags" array.
func (sc *scanner) tags(sh *Sheet) error {
	return sc.array(func() error {
		if len(sh.Tags) >= maxSheetTags {
			return ErrSheetTooMany
		}
		var (
			t    Tag
			from float64
			to   float64
		)
		err := sc.object(func(key string) error {
			switch key {
			case "name":
				return sc.str(&t.Name)
			case "from":
				return sc.number(&from)
			case "to":
				return sc.number(&to)
			case "direction":
				var d string
				if err := sc.str(&d); err != nil {
					return err
				}
				t.Direction = direction(d)
				return nil
			}
			return sc.skip()
		})
		if err != nil {
			return err
		}
		if !finite(from) || !finite(to) {
			return ErrSheetRange
		}
		// The index has to survive the round trip through float64 that JSON
		// numbers arrive as, so a fractional or enormous one is refused here
		// rather than silently truncated into a valid-looking frame.
		t.From, t.To = int(from), int(to)
		if float64(t.From) != from || float64(t.To) != to {
			return ErrSheetRange
		}
		sh.Tags = append(sh.Tags, t)
		return nil
	})
}

// direction maps Aseprite's spelling to a [Direction]. Anything unknown plays
// forward, which is what a new Aseprite mode should degrade to rather than
// failing an export that is otherwise fine.
func direction(s string) Direction {
	switch s {
	case "reverse":
		return Reverse
	case "pingpong":
		return PingPong
	}
	return Forward
}

// scanner walks the bytes once, left to right, keeping only what it was asked
// for. depth is what stops a nested file from nesting the parser's own stack.
type scanner struct {
	s     []byte
	i     int
	depth int
}

// ws steps over the four bytes JSON calls whitespace.
func (sc *scanner) ws() {
	for sc.i < len(sc.s) {
		switch sc.s[sc.i] {
		case ' ', '\t', '\n', '\r':
			sc.i++
		default:
			return
		}
	}
}

// peek returns the next byte, or 0 at the end of the input. Zero is not a byte
// any of the callers accept, so the end fails the same test a wrong byte does.
func (sc *scanner) peek() byte {
	sc.ws()
	if sc.i >= len(sc.s) {
		return 0
	}
	return sc.s[sc.i]
}

// atEnd reports whether nothing but whitespace is left. Trailing bytes mean
// the file is two documents, or one and some rubble.
func (sc *scanner) atEnd() error {
	if sc.ws(); sc.i != len(sc.s) {
		return ErrSheetSyntax
	}
	return nil
}

// enter and leave keep the depth count. Every object and array passes through
// them, including the ones being skipped.
func (sc *scanner) enter() error {
	sc.depth++
	if sc.depth > maxSheetDepth {
		return ErrSheetTooDeep
	}
	return nil
}

func (sc *scanner) leave() { sc.depth-- }

// object reads an object and calls fn with each key. fn must consume the key's
// value — by reading it, or by calling [scanner.skip].
func (sc *scanner) object(fn func(key string) error) error {
	if sc.peek() != '{' {
		return ErrSheetSyntax
	}
	if err := sc.enter(); err != nil {
		return err
	}
	defer sc.leave()
	sc.i++

	if sc.peek() == '}' {
		sc.i++
		return nil
	}
	for {
		var key string
		if err := sc.str(&key); err != nil {
			return err
		}
		if sc.peek() != ':' {
			return ErrSheetSyntax
		}
		sc.i++
		if err := fn(key); err != nil {
			return err
		}
		switch sc.peek() {
		case ',':
			sc.i++
		case '}':
			sc.i++
			return nil
		default:
			return ErrSheetSyntax
		}
	}
}

// array reads an array and calls fn once per element, which must consume it.
func (sc *scanner) array(fn func() error) error {
	if sc.peek() != '[' {
		return ErrSheetSyntax
	}
	if err := sc.enter(); err != nil {
		return err
	}
	defer sc.leave()
	sc.i++

	if sc.peek() == ']' {
		sc.i++
		return nil
	}
	for {
		if err := fn(); err != nil {
			return err
		}
		switch sc.peek() {
		case ',':
			sc.i++
		case ']':
			sc.i++
			return nil
		default:
			return ErrSheetSyntax
		}
	}
}

// skip consumes one value of any kind and keeps nothing.
func (sc *scanner) skip() error {
	switch c := sc.peek(); c {
	case '{':
		return sc.object(func(string) error { return sc.skip() })
	case '[':
		return sc.array(sc.skip)
	case '"':
		var s string
		return sc.str(&s)
	case 't':
		return sc.lit("true")
	case 'f':
		return sc.lit("false")
	case 'n':
		return sc.lit("null")
	default:
		var f float64
		return sc.number(&f)
	}
}

// lit consumes one of the three bare words JSON has.
func (sc *scanner) lit(word string) error {
	sc.ws()
	if sc.i+len(word) > len(sc.s) || string(sc.s[sc.i:sc.i+len(word)]) != word {
		return ErrSheetSyntax
	}
	sc.i += len(word)
	return nil
}

// number reads a JSON number into f.
//
// The extent is scanned permissively and strconv decides whether it was a
// number, which is both shorter than the grammar and stricter: "1.2.3" is
// refused by the same call that turns 1e400 into an infinity for [Sheet.check]
// to reject.
func (sc *scanner) number(f *float64) error {
	sc.ws()
	start := sc.i
	for sc.i < len(sc.s) {
		switch c := sc.s[sc.i]; {
		case c >= '0' && c <= '9', c == '-', c == '+', c == '.', c == 'e', c == 'E':
			sc.i++
		default:
			goto done
		}
	}
done:
	if sc.i == start {
		return ErrSheetSyntax
	}
	v, err := strconv.ParseFloat(string(sc.s[start:sc.i]), 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return ErrSheetSyntax
	}
	*f = v
	return nil
}

// str reads a JSON string into out, undoing the escapes.
func (sc *scanner) str(out *string) error {
	if sc.peek() != '"' {
		return ErrSheetSyntax
	}
	sc.i++

	// The common string has no escape in it at all, so the bytes are handed
	// over whole rather than one at a time into a builder.
	start := sc.i
	for sc.i < len(sc.s) {
		c := sc.s[sc.i]
		switch {
		case c == '"':
			*out = string(sc.s[start:sc.i])
			sc.i++
			return nil
		case c == '\\':
			return sc.strEscaped(out, start)
		case c < 0x20:
			return ErrSheetSyntax
		}
		sc.i++
	}
	return ErrSheetSyntax
}

// strEscaped finishes a string that turned out to have an escape in it. done
// is where the string started, so the plain run before the first backslash is
// still copied in one go.
func (sc *scanner) strEscaped(out *string, start int) error {
	buf := make([]byte, 0, len(sc.s)-start)
	buf = append(buf, sc.s[start:sc.i]...)

	for sc.i < len(sc.s) {
		c := sc.s[sc.i]
		switch {
		case c == '"':
			sc.i++
			*out = string(buf)
			return nil
		case c < 0x20:
			return ErrSheetSyntax
		case c != '\\':
			buf = append(buf, c)
			sc.i++
			continue
		}

		sc.i++
		if sc.i >= len(sc.s) {
			return ErrSheetSyntax
		}
		switch sc.s[sc.i] {
		case '"', '\\', '/':
			buf = append(buf, sc.s[sc.i])
		case 'b':
			buf = append(buf, '\b')
		case 'f':
			buf = append(buf, '\f')
		case 'n':
			buf = append(buf, '\n')
		case 'r':
			buf = append(buf, '\r')
		case 't':
			buf = append(buf, '\t')
		case 'u':
			r, err := sc.escapedRune()
			if err != nil {
				return err
			}
			buf = utf8.AppendRune(buf, r)
			continue
		default:
			return ErrSheetSyntax
		}
		sc.i++
	}
	return ErrSheetSyntax
}

// escapedRune reads a \u escape, and the low half of a surrogate pair when the
// first half turns out to be the high one. It is entered on the 'u' and leaves
// on the byte after the last hex digit.
//
// A half a pair with no partner becomes the replacement character rather than
// an error: it is a name to show a player, not a number to compute with.
func (sc *scanner) escapedRune() (rune, error) {
	r, err := sc.hex4()
	if err != nil {
		return 0, err
	}
	if !utf16.IsSurrogate(r) {
		return r, nil
	}
	if sc.i+1 < len(sc.s) && sc.s[sc.i] == '\\' && sc.s[sc.i+1] == 'u' {
		// Step onto the 'u', which is where hex4 starts from, not past it.
		sc.i++
		lo, err := sc.hex4()
		if err != nil {
			return 0, err
		}
		if dec := utf16.DecodeRune(r, lo); dec != utf8.RuneError {
			return dec, nil
		}
	}
	return utf8.RuneError, nil
}

// hex4 reads the four hex digits of a \u escape. It is entered on the 'u'.
func (sc *scanner) hex4() (rune, error) {
	if sc.i+4 >= len(sc.s) {
		return 0, ErrSheetSyntax
	}
	var r rune
	for _, c := range sc.s[sc.i+1 : sc.i+5] {
		switch {
		case c >= '0' && c <= '9':
			r = r<<4 | rune(c-'0')
		case c >= 'a' && c <= 'f':
			r = r<<4 | rune(c-'a'+10)
		case c >= 'A' && c <= 'F':
			r = r<<4 | rune(c-'A'+10)
		default:
			return 0, ErrSheetSyntax
		}
	}
	sc.i += 5
	return r, nil
}
