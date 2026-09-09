package wisp

// A sheet is one image holding every frame of every animation, and a
// description of where those frames are. The engine understands two ways of
// saying it and treats them as one thing.
//
// The grid convention is the older one and needs no description at all: every
// frame is the entity's own width and height, animations are rows, and
// [AnimationSettings] says how many frames a row holds and how long each one
// lasts. It costs nothing to state and it cannot describe anything else.
//
// A [Sheet] is the other, and it is what Aseprite exports: a rectangle and a
// duration per frame, and named ranges over them. [GridSheet] builds the grid
// convention as a Sheet, so the two are the same code path with two ways in
// rather than two renderers to keep in step.

// Direction is the order a [Tag] plays its frames in. It matches Aseprite's
// own three, and it is spelled the way the export spells it.
type Direction uint8

// The play orders. Forward is the zero value, so a Tag nobody set a direction
// on plays the way almost every animation does.
const (
	Forward Direction = iota
	Reverse
	PingPong
)

// Frame is one frame: where it is on the image, and how long it shows.
//
// The rectangle is in image pixels, which is why a packed or trimmed sheet
// works without the engine knowing it is one. Duration is milliseconds.
type Frame struct {
	// Duration is how long this frame shows, in milliseconds.
	Duration float64
	// H is the frame's height on the image.
	H float64
	// W is the frame's width on the image.
	W float64
	// X is the frame's left edge on the image.
	X float64
	// Y is the frame's top edge on the image.
	Y float64
}

// Tag is one named animation: a run of frames and the order to play them in.
//
// From and To are both inclusive indices into [Sheet].Frames, which is how
// Aseprite writes them. A tag of one frame has From equal to To.
type Tag struct {
	// Direction is the play order.
	Direction Direction
	// From is the first frame of the animation.
	From int
	// Name is what the animation is called in Aseprite. [Engine.Play] takes
	// this. A grid sheet has no names, so its tags carry an empty one.
	Name string
	// To is the last frame of the animation, and it is included.
	To int
}

// Sheet is a parsed sprite sheet: every frame on one image, and the named
// animations over them.
//
// Attach one per image through [Engine.Sheets]. The zero Sheet means the grid
// convention, so an image nobody described keeps drawing the way it always
// did.
//
// [ParseSheet] and [GridSheet] both return a Sheet the draw can trust, and
// they are the two ways to get one. A Sheet built by hand must hold up the
// same promise — every tag's From and To index Frames — because the draw reads
// a frame per entity per pass and checking it there would be a check per
// sprite per frame. Breaking it panics, the way an out-of-range index into any
// of the entity arrays does.
type Sheet struct {
	// Frames is every frame on the image, in the order the export lists them.
	Frames []Frame
	// H is the image's height, as the export measured it. The parser checks
	// every frame against it, so a rectangle can never point off the image.
	H float64
	// Tags is the named animations. [Engine.ImageRow] indexes it.
	Tags []Tag
	// W is the image's width, as the export measured it.
	W float64
}

// GridSheet builds the grid convention as a [Sheet]: cols frames of w by h
// pixels per row, rows rows, every frame lasting ms milliseconds.
//
// It exists to prove the two descriptions are one. A game on the grid does not
// need it — leaving [Engine.Sheets] empty draws the same pixels — but a game
// moving to Aseprite can build the sheet it already had and compare.
//
// The tags are the rows, in order and unnamed, because a grid says nothing
// about what a row is for. Naming them is what the export adds.
func GridSheet(cols, rows int, w, h, ms float64) Sheet {
	if cols <= 0 || rows <= 0 {
		return Sheet{}
	}
	s := Sheet{
		Frames: make([]Frame, 0, cols*rows),
		H:      float64(rows) * h,
		Tags:   make([]Tag, 0, rows),
		W:      float64(cols) * w,
	}
	for r := range rows {
		for c := range cols {
			s.Frames = append(s.Frames, Frame{
				Duration: ms,
				H:        h,
				W:        w,
				X:        float64(c) * w,
				Y:        float64(r) * h,
			})
		}
		s.Tags = append(s.Tags, Tag{From: r * cols, To: r*cols + cols - 1})
	}
	return s
}

// Lookup returns the index of the tag named name, or -1 when the sheet has no
// such animation.
//
// It is a scan, because a sheet holds a handful of tags and a map would cost
// more to build than it ever saves. Resolve a name once and keep the index if
// a game plays the same animation every frame.
func (s *Sheet) Lookup(name string) int {
	for i := range s.Tags {
		if s.Tags[i].Name == name {
			return i
		}
	}
	return -1
}

// Play starts the animation named name on entity i, from its first frame, and
// reports whether the sheet had it.
//
// It writes the tag's index into [Engine.ImageRow], which is the same place
// RowForState writes a row: an animation is an animation, whether a name or a
// state bit picked it. Nothing happens and it returns false when the entity's
// image has no sheet, or the sheet has no such tag — a game may then fall back
// to a row it does know.
//
// Play does not make the entity animate. Set StateAnimated, and
// StateAnimatedLoop for one that repeats, the way the grid path does.
func (e *Engine) Play(i int, name string) bool {
	s := e.sheetFor(i)
	if s == nil {
		return false
	}
	tag := s.Lookup(name)
	if tag < 0 {
		return false
	}
	e.ImageRow[i] = tag
	e.FrameOffset[i] = 0
	e.FrameTime[i] = 0
	return true
}

// Playing returns the name of the animation entity i is on, or "" when its
// image has no sheet or the row is not a tag.
func (e *Engine) Playing(i int) string {
	s := e.sheetFor(i)
	if s == nil {
		return ""
	}
	row := e.ImageRow[i]
	if row < 0 || row >= len(s.Tags) {
		return ""
	}
	return s.Tags[row].Name
}

// sheetFor returns the sheet describing entity i's image, or nil when there is
// none and the grid convention applies.
//
// A Sheet with no frames counts as none, so a game may size [Engine.Sheets] to
// its images and fill in only the ones it has exported.
func (e *Engine) sheetFor(i int) *Sheet {
	if i < 0 || i >= len(e.ImageIndex) {
		return nil
	}
	img := e.ImageIndex[i]
	if img < 0 || img >= len(e.Sheets) {
		return nil
	}
	s := &e.Sheets[img]
	if len(s.Frames) == 0 || len(s.Tags) == 0 {
		return nil
	}
	return s
}

// cycle returns how many steps entity i's current animation takes before it
// starts over, and how long the frame it is showing lasts.
//
// The step count is not the frame count for a ping-pong tag: eight frames
// there is fourteen steps, because the two ends are not repeated. Everything
// that advances or ends an animation counts steps, so a one-shot ping-pong
// stops after the way back rather than at the far end.
func (e *Engine) cycle(i int) (steps int, dur float64) {
	s := e.sheetFor(i)
	if s == nil {
		return e.Animation.FrameCount, e.Animation.FrameDuration
	}
	row := e.ImageRow[i]
	if row < 0 || row >= len(s.Tags) {
		return e.Animation.FrameCount, e.Animation.FrameDuration
	}
	t := s.Tags[row]
	n := t.To - t.From + 1
	steps = n
	if t.Direction == PingPong && n > 1 {
		steps = 2*n - 2
	}
	return steps, s.Frames[frameAt(t, n, e.FrameOffset[i])].Duration
}

// frameAt maps a step of a tag's play order to the frame it shows. n is the
// tag's frame count, passed in because every caller already has it.
//
// This is what makes Reverse and PingPong need no per-entity state. The offset
// counts steps taken, always upwards; the direction decides which frame a step
// lands on, so nothing has to remember which way it was going.
func frameAt(t Tag, n, offset int) int {
	if n <= 1 || offset <= 0 {
		if t.Direction == Reverse {
			return t.To
		}
		return t.From
	}
	switch t.Direction {
	case Reverse:
		return t.To - offset%n
	case PingPong:
		if steps := 2*n - 2; offset%steps >= n {
			return t.From + (2*n - 2 - offset%steps)
		}
		return t.From + offset%n
	default:
		return t.From + offset%n
	}
}

// srcRect returns the rectangle on the image that entity i's current frame
// occupies.
//
// It is the one place the two sheet descriptions meet, so the draw asks a
// question rather than doing arithmetic it would have to do twice.
func (e *Engine) srcRect(i int) (x, y, w, h float64) {
	s := e.sheetFor(i)
	if s == nil {
		// The grid: the column picks the sprite, the offset picks the frame,
		// and the row picks the animation.
		w, h = e.SpriteWidth[i], e.SpriteHeight[i]
		return float64(e.ImageColumn[i]+e.FrameOffset[i]) * w,
			float64(e.ImageRow[i]) * h, w, h
	}
	row := e.ImageRow[i]
	if row < 0 || row >= len(s.Tags) {
		w, h = e.SpriteWidth[i], e.SpriteHeight[i]
		return float64(e.ImageColumn[i]+e.FrameOffset[i]) * w,
			float64(e.ImageRow[i]) * h, w, h
	}
	t := s.Tags[row]
	f := s.Frames[frameAt(t, t.To-t.From+1, e.FrameOffset[i])]
	return f.X, f.Y, f.W, f.H
}
