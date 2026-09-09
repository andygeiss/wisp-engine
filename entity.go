package wisp

import "slices"

// Sprite describes an entity at the moment it is added. Its zero value is a
// 0x0 invisible entity at the origin, so fill in what matters and leave the
// rest.
type Sprite struct {
	// Alpha is the opacity, 0 to 1. Because an entity added invisible is
	// almost always a mistake, 0 here means 1; set e.Alpha[i] afterwards for
	// an entity that really should start transparent.
	Alpha float64
	// Column is the sheet column the sprite starts at. The animation frame is
	// added to it, so one sheet holds every animation of every sprite.
	Column int
	// Height of the sprite in pixels. It is also the sheet's cell height.
	Height float64
	// Image is which loaded image, in [Engine.LoadImages] order.
	Image int
	// Row is the sheet row, which is one animation.
	Row int
	// ScreenSpace draws the sprite in canvas pixels, past the camera. Use it
	// for a HUD.
	ScreenSpace bool
	// SpeedFactor multiplies [WorldSettings.Speed] for this entity. 0 means 1.
	SpeedFactor float64
	// State is the entity's state bits.
	State uint64
	// Width of the sprite in pixels. It is also the sheet's cell width.
	Width float64
	// X is the world x of the sprite's centre.
	X float64
	// Y is the world y of the sprite's centre.
	Y float64
	// Z is the draw layer. Lower draws first.
	Z int
}

// ID names one entity for as long as it lives, and never names another one.
//
// An index is a position in the entity arrays. It is stable — a delete leaves
// its slot where it is — but it is not unique for ever, because the next Add
// reuses the slot. An ID carries the generation of the entity that filled it
// as well, so an ID left over from the entity before resolves to nothing
// rather than to a stranger.
//
// Hold an ID when the entity has to survive being deleted by somebody else:
// across frames, between a game and its server, inside a packet. Do the work
// through the index it resolves to, which is what every array is keyed by, and
// resolve once rather than in a loop.
//
// The zero ID names nothing.
type ID uint64

// Add adds an entity and returns its index. The sprite is drawn centred on
// (X, Y).
//
// The index is stable: it keeps meaning this entity until the entity is
// deleted. It is reused after that, so an index held across a delete needs an
// [ID] instead — see [Engine.IDOf].
func (e *Engine) Add(s Sprite) (index int) {
	if s.Alpha == 0 {
		s.Alpha = 1
	}
	if s.SpeedFactor == 0 {
		s.SpeedFactor = 1
	}
	index = e.claim()
	e.set(index, s)
	e.gen[index] = e.nextGen
	e.drawOrder = append(e.drawOrder, index)
	return index
}

// IDOf returns the ID of entity i, or the zero ID when i holds no entity.
func (e *Engine) IDOf(i int) ID {
	if !e.Live(i) {
		return 0
	}
	return ID(uint64(e.gen[i])<<32 | uint64(uint32(i)))
}

// Index returns the index id names, or -1 when that entity is gone — deleted,
// or cleared by [Engine.Reset]. A slot reused since is gone too, which is the
// whole reason an ID is not just an index.
func (e *Engine) Index(id ID) int {
	i := int(uint32(id))
	if !e.Live(i) || e.gen[i] != uint32(id>>32) {
		return -1
	}
	return i
}

// claim returns a slot to write an entity into, reusing a deleted one when
// there is one and growing the arrays when there is not.
//
// Reuse is what makes an index stable. Compacting the arrays on every delete
// moved every entity above the hole, so an index a game was holding silently
// came to mean its neighbour — and an index in a packet could not mean
// anything at all by the time the packet arrived.
func (e *Engine) claim() (index int) {
	// One counter for the whole engine rather than one per slot, so an ID
	// stays unique across [Engine.Reset] too. It wraps after four billion
	// entities, which is longer than a browser tab lives.
	e.nextGen++
	e.live++

	if n := len(e.free); n > 0 {
		index = e.free[n-1]
		e.free = e.free[:n-1]
		e.alive[index] = true
		return index
	}

	index = len(e.State)
	e.Alpha = append(e.Alpha, 0)
	e.FrameOffset = append(e.FrameOffset, 0)
	e.FrameTime = append(e.FrameTime, 0)
	e.ImageColumn = append(e.ImageColumn, 0)
	e.ImageIndex = append(e.ImageIndex, 0)
	e.ImageRow = append(e.ImageRow, 0)
	e.ScreenSpace = append(e.ScreenSpace, false)
	e.SpeedFactor = append(e.SpeedFactor, 0)
	e.SpriteHeight = append(e.SpriteHeight, 0)
	e.SpriteWidth = append(e.SpriteWidth, 0)
	e.State = append(e.State, 0)
	e.X = append(e.X, 0)
	e.Y = append(e.Y, 0)
	e.Z = append(e.Z, 0)
	e.alive = append(e.alive, true)
	e.gen = append(e.gen, 0)
	return index
}

// set writes sprite s into slot i, which every array already has room for. Add
// fills a slot with it and Delete empties one with the zero Sprite, so the two
// halves of a slot's life are one list of fields rather than two.
func (e *Engine) set(i int, s Sprite) {
	e.Alpha[i] = s.Alpha
	e.FrameOffset[i] = 0
	e.FrameTime[i] = 0
	e.ImageColumn[i] = s.Column
	e.ImageIndex[i] = s.Image
	e.ImageRow[i] = s.Row
	e.ScreenSpace[i] = s.ScreenSpace
	e.SpeedFactor[i] = s.SpeedFactor
	e.SpriteHeight[i] = s.Height
	e.SpriteWidth[i] = s.Width
	e.State[i] = s.State
	e.X[i] = s.X
	e.Y[i] = s.Y
	e.Z[i] = s.Z
}

// Tilemap describes a grid of tiles to add as entities, one per cell.
type Tilemap struct {
	// Height of one tile in pixels.
	Height float64
	// Image is which loaded image the tileset comes from.
	Image int
	// Rows and Cols are the shape of the map.
	Cols int
	Rows int
	// TilesetCols and TilesetRows are the shape of the tileset image, in
	// tiles. A tile index outside them leaves the cell empty.
	TilesetCols int
	TilesetRows int
	// Tiles is one tile index per cell, row by row. A negative index leaves
	// the cell empty, and a short slice fills what it can.
	Tiles []int
	// Width of one tile in pixels.
	Width float64
	// Z is the draw layer every tile lands on. A floor belongs below the
	// entities that stand on it: inside one layer the draw order is by
	// baseline, so a tile sharing a layer with an actor draws over it
	// whenever the tile sits lower down the screen.
	Z int
}

// AddTilemap adds one visible entity per tile. Tiles do not move and carry no
// state bits, so they cost a draw and nothing else.
func (e *Engine) AddTilemap(m Tilemap) {
	maxTile := m.TilesetCols * m.TilesetRows
	for row := range m.Rows {
		for col := range m.Cols {
			idx := row*m.Cols + col
			if idx >= len(m.Tiles) {
				return
			}
			t := m.Tiles[idx]
			if t < 0 || t >= maxTile {
				continue
			}
			// Sprites draw centred, so a tile sits at the centre of its cell.
			e.Add(Sprite{
				Column: t % m.TilesetCols,
				Height: m.Height,
				Image:  m.Image,
				Row:    t / m.TilesetCols,
				State:  StateVisible,
				Width:  m.Width,
				X:      float64(col)*m.Width + m.Width/2,
				Y:      float64(row)*m.Height + m.Height/2,
				Z:      m.Z,
			})
		}
	}
}

// BoundingBox returns the left, top, right and bottom edge of entity i's hit
// box: the sprite shrunk by [WorldSettings.HitBoxMargin] on all four sides, so
// sprites have to visibly overlap before they collide. A margin past half the
// sprite gives a box of zero size, which never collides.
//
// It panics when i is not an entity, which is a programmer error.
func (e *Engine) BoundingBox(i int) (l, t, r, b float64) {
	m := e.World.HitBoxMargin
	hw := max(e.SpriteWidth[i]/2-m, 0)
	hh := max(e.SpriteHeight[i]/2-m, 0)
	return e.X[i] - hw, e.Y[i] - hh, e.X[i] + hw, e.Y[i] + hh
}

// Count returns how many entities there are.
//
// It is not the range to iterate. A deleted entity leaves its slot behind, so
// the arrays can be longer than the count — use [Engine.Slots] for the range
// and [Engine.Live] to skip the holes.
func (e *Engine) Count() int { return e.live }

// Slots returns the length of every entity array, which is the range to
// iterate:
//
//	for i := range e.Slots() {
//		if !e.Live(i) {
//			continue
//		}
//		...
//	}
//
// It only ever grows while entities exist, because a deleted entity's slot is
// kept for the next one. [Engine.Reset] returns it to zero.
func (e *Engine) Slots() int { return len(e.State) }

// Live reports whether slot i holds an entity. It is false for a deleted slot
// and for an index outside the arrays, so it is safe to ask about anything.
func (e *Engine) Live(i int) bool { return i >= 0 && i < len(e.alive) && e.alive[i] }

// Delete removes entity i and keeps its slot for the next [Engine.Add]. No
// other index moves, so a game — or a packet — may hold an index across a
// delete of somebody else.
//
// The slot is emptied rather than compacted away: it becomes a 0x0 invisible
// entity with no state bits, which the update and the draw skip on the tests
// they already do. Deleting the same entity twice does nothing the second
// time, so a game holding an index does not have to track whether it already
// used it. CamTarget and InputTarget are cleared to -1 when they point here.
//
// It panics when i is outside the arrays, which is a programmer error.
func (e *Engine) Delete(i int) {
	if !e.alive[i] {
		return
	}
	e.set(i, Sprite{})
	e.alive[i] = false
	e.free = append(e.free, i)
	e.live--

	// drawOrder holds indices, and no index moved, so only this one goes.
	if n := slices.Index(e.drawOrder, i); n >= 0 {
		e.drawOrder = slices.Delete(e.drawOrder, n, n+1)
	}

	if e.CamTarget == i {
		e.CamTarget = -1
	}
	if e.InputTarget == i {
		e.InputTarget = -1
	}
}

// HasCollision reports whether the hit boxes of entities i and j overlap.
func (e *Engine) HasCollision(i, j int) bool {
	il, it, ir, ib := e.BoundingBox(i)
	jl, jt, jr, jb := e.BoundingBox(j)
	return il < jr && ir > jl && it < jb && ib > jt
}

// Reset removes every entity, keeping the capacity the slices have grown to. A
// game calls it once per scene. It leaves the settings and the camera alone.
//
// Every [ID] handed out before it stops resolving, so a stale one cannot come
// back to life as an entity in the new scene.
func (e *Engine) Reset() {
	e.Alpha = e.Alpha[:0]
	e.FrameOffset = e.FrameOffset[:0]
	e.FrameTime = e.FrameTime[:0]
	e.ImageColumn = e.ImageColumn[:0]
	e.ImageIndex = e.ImageIndex[:0]
	e.ImageRow = e.ImageRow[:0]
	e.ScreenSpace = e.ScreenSpace[:0]
	e.SpeedFactor = e.SpeedFactor[:0]
	e.SpriteHeight = e.SpriteHeight[:0]
	e.SpriteWidth = e.SpriteWidth[:0]
	e.State = e.State[:0]
	e.X = e.X[:0]
	e.Y = e.Y[:0]
	e.Z = e.Z[:0]
	e.alive = e.alive[:0]
	e.gen = e.gen[:0]
	e.free = e.free[:0]
	e.live = 0
	e.drawOrder = e.drawOrder[:0]
	e.CamTarget, e.InputTarget = -1, -1
}
