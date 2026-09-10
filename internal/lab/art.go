package lab

import "github.com/andygeiss/wisp-engine"

// The art is PixelLab's, by the IDs in assets/pixellab.txt, and "make art"
// writes it under web/static/img. What this file holds is the shape of those
// files as constants the browser module can carry — a cell size, a column
// count, the order of the rows — and a test in this package reads the
// committed JSON and PNGs and checks every one of them, so a regenerated
// asset cannot quietly change shape under the code that draws it.

// Images, in the order the client loads them and a Spawn names them.
const (
	ImageHero = iota
	ImageFox
	ImageMeadow
	ImagePath
	ImageTree
	ImageBoulder
	ImageBush
	ImageStump
	NumImages
)

// Images is the file behind each image, under the page's static tree.
var Images = [NumImages]string{
	"/static/img/hero.png",
	"/static/img/fox.png",
	"/static/img/meadow.png",
	"/static/img/path.png",
	"/static/img/tree.png",
	"/static/img/boulder.png",
	"/static/img/bush.png",
	"/static/img/stump.png",
}

// The sprites' sizes in pixels: a character's is the cell cmd/art cropped
// its sheet to, a prop's is its PNG. The engine draws a sprite at its size,
// takes its hit box from it and sorts by its bottom edge, which is why the
// cells are cropped to the figure rather than left at PixelLab's canvas.
const (
	HeroW, HeroH       = 52, 48
	FoxW, FoxH         = 44, 34
	TreeW, TreeH       = 42, 56
	BoulderW, BoulderH = 28, 28
	BushW, BushH       = 26, 23
	StumpW, StumpH     = 24, 24
)

// The eight directions, in the order PixelLab lays them out: south first,
// then clockwise. The index is what the sheets and the bouncers use.
const (
	South = iota
	SouthEast
	East
	NorthEast
	North
	NorthWest
	West
	SouthWest
	NumDirections
)

// Directions names each direction the way the export does.
var Directions = [NumDirections]string{
	"south", "south-east", "east", "north-east", "north", "north-west", "west", "south-west",
}

// DirectionBits is each direction as facing bits. The engine at Facing 8
// sets one bit from each axis, so a diagonal is two of them, and these are
// the keys the row map is built on.
var DirectionBits = [NumDirections]uint64{
	wisp.StateFaceDown,
	wisp.StateFaceDown | wisp.StateFaceRight,
	wisp.StateFaceRight,
	wisp.StateFaceUp | wisp.StateFaceRight,
	wisp.StateFaceUp,
	wisp.StateFaceUp | wisp.StateFaceLeft,
	wisp.StateFaceLeft,
	wisp.StateFaceDown | wisp.StateFaceLeft,
}

// DirectionOf is the direction a velocity travels in, eight ways: the
// nearest of the four axes and the four diagonals, with y growing downwards
// the way the screen does. A velocity of zero faces south, which is the
// way a sprite is drawn when nothing else is known.
func DirectionOf(vx, vy float64) int {
	ax, ay := vx, vy
	if ax < 0 {
		ax = -ax
	}
	if ay < 0 {
		ay = -ay
	}
	if ax == 0 && ay == 0 {
		return South
	}
	// tan(22.5°): below it the other axis does not count.
	const edge = 0.41421356
	horizontal, vertical := ay <= ax*edge, ax <= ay*edge
	switch {
	case horizontal && vx > 0:
		return East
	case horizontal:
		return West
	case vertical && vy > 0:
		return South
	case vertical:
		return North
	case vx > 0 && vy > 0:
		return SouthEast
	case vx > 0:
		return NorthEast
	case vy > 0:
		return SouthWest
	}
	return NorthWest
}

// The animations of each sheet, in the order cmd/art lays them out: the
// rotations first, one row of eight standing poses, then every animation in
// alphabetical order of its name, eight rows each, one a direction. Each
// sheet has its own list, because an animation's index is its place on that
// sheet: the fox's walk is its first, the ranger's its third. A tag's index
// is [Tag] of one of these, so a row map holds tag indices computed rather
// than typed.
const (
	HeroFace = iota
	HeroIdle
	HeroStrike
	HeroWalk
	NumHeroAnims
)

// The fox stands and walks.
const (
	FoxFace = iota
	FoxWalk
	NumFoxAnims
)

// The animations' names, the way the export names them and a tag is called;
// the rotations row has no name of its own, so the first is the engine's.
var (
	heroAnims = [NumHeroAnims]string{"face", "idle", "strike", "walk"}
	foxAnims  = [NumFoxAnims]string{"face", "walk"}
)

// The sheets' shapes: the columns each has, which is its widest row, and
// the frames of each animation. FrameMS is what every frame lasts, because
// PixelLab's export carries no durations.
const (
	Columns      = 8
	FramesWalk   = 8
	FramesStrike = 8
	FramesIdle   = 4
	FrameMS      = 100
)

// heroFrames and foxFrames are the frames per animation, by animation.
var (
	heroFrames = [NumHeroAnims]int{1, FramesIdle, FramesStrike, FramesWalk}
	foxFrames  = [NumFoxAnims]int{1, FramesWalk}
)

// Tag returns the tag index of a sheet's animation anim facing direction d,
// which is what [wisp.Engine.ImageRow] holds when the image has a sheet.
func Tag(anim, d int) int { return anim*NumDirections + d }

// HeroSheet is the ranger's sheet: the rotations, then idle, strike and
// walk in eight directions each.
func HeroSheet() wisp.Sheet { return sheet(HeroW, HeroH, heroAnims[:], heroFrames[:]) }

// FoxSheet is the fox's sheet: the rotations, then a walk.
func FoxSheet() wisp.Sheet { return sheet(FoxW, FoxH, foxAnims[:], foxFrames[:]) }

// sheet builds a PixelLab spritesheet, as cmd/art writes it, as the engine's
// [wisp.Sheet]: the grid of cells, and one tag per animation and direction
// over it, named "<animation>-<direction>". The rotations row holds eight
// one-frame tags side by side; every other animation is eight rows of
// frames, one row a direction, in [Directions] order.
func sheet(w, h int, names []string, frames []int) wisp.Sheet {
	rows := 1 + (len(frames)-1)*NumDirections
	s := wisp.GridSheet(Columns, rows, float64(w), float64(h), FrameMS)
	s.Tags = s.Tags[:0]
	for d := range NumDirections {
		s.Tags = append(s.Tags, wisp.Tag{Name: names[0] + "-" + Directions[d], From: d, To: d})
	}
	for anim := 1; anim < len(frames); anim++ {
		for d := range NumDirections {
			from := (1 + (anim-1)*NumDirections + d) * Columns
			s.Tags = append(s.Tags, wisp.Tag{
				Name: names[anim] + "-" + Directions[d], From: from, To: from + frames[anim] - 1,
			})
		}
	}
	return s
}

// Rows maps a player's pose to the tag that draws it: idle, walking or
// striking, in each of the eight directions.
func Rows() map[uint64]int {
	rows := make(map[uint64]int, 3*NumDirections)
	for d := range NumDirections {
		rows[DirectionBits[d]|wisp.StateIdle] = Tag(HeroIdle, d)
		rows[DirectionBits[d]|wisp.StateMove] = Tag(HeroWalk, d)
		rows[DirectionBits[d]|StateStrike] = Tag(HeroStrike, d)
	}
	return rows
}

// Prop is a thing standing on the meadow: which image, its size, and where
// its centre is in world pixels. Props collide with nothing, so the server
// never has them; each client stands them up beside the floor.
type Prop struct {
	Image int
	W, H  float64
	X, Y  float64
}

// Props is what stands on the meadow, off the lake and the path.
var Props = []Prop{
	{ImageTree, TreeW, TreeH, 160, 150},
	{ImageTree, TreeW, TreeH, 330, 90},
	{ImageTree, TreeW, TreeH, 900, 120},
	{ImageTree, TreeW, TreeH, 560, 700},
	{ImageBoulder, BoulderW, BoulderH, 250, 500},
	{ImageBoulder, BoulderW, BoulderH, 1180, 100},
	{ImageBush, BushW, BushH, 1180, 420},
	{ImageBush, BushW, BushH, 100, 560},
	{ImageStump, StumpW, StumpH, 1200, 700},
	{ImageStump, StumpW, StumpH, 420, 100},
}

// BuildProps stands every prop up, on the actors' layer so that a hero
// walking behind a tree is drawn behind it.
func BuildProps(e *wisp.Engine) {
	for _, p := range Props {
		e.Add(wisp.Sprite{
			Height: p.H, Image: p.Image, State: wisp.StateVisible,
			Width: p.W, X: p.X, Y: p.Y, Z: ZProps,
		})
	}
}
