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

// Add adds an entity and returns its index. The sprite is drawn centred on
// (X, Y).
func (e *Engine) Add(s Sprite) (index int) {
	if s.Alpha == 0 {
		s.Alpha = 1
	}
	if s.SpeedFactor == 0 {
		s.SpeedFactor = 1
	}
	index = len(e.State)
	e.Alpha = append(e.Alpha, s.Alpha)
	e.FrameOffset = append(e.FrameOffset, 0)
	e.FrameTime = append(e.FrameTime, 0)
	e.ImageColumn = append(e.ImageColumn, s.Column)
	e.ImageIndex = append(e.ImageIndex, s.Image)
	e.ImageRow = append(e.ImageRow, s.Row)
	e.ScreenSpace = append(e.ScreenSpace, s.ScreenSpace)
	e.SpeedFactor = append(e.SpeedFactor, s.SpeedFactor)
	e.SpriteHeight = append(e.SpriteHeight, s.Height)
	e.SpriteWidth = append(e.SpriteWidth, s.Width)
	e.State = append(e.State, s.State)
	e.X = append(e.X, s.X)
	e.Y = append(e.Y, s.Y)
	e.Z = append(e.Z, s.Z)
	e.drawOrder = append(e.drawOrder, index)
	return index
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
	// Z is the draw layer every tile lands on.
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
// box: the sprite shrunk by [CollisionSettings] Margin on all four sides, so
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

// Count returns how many entities there are. Every index below it is valid.
func (e *Engine) Count() int { return len(e.State) }

// Delete removes entity i. Every index above i shifts down by one, so a game
// holding indices must shift them too; CamTarget and InputTarget are shifted
// here. Hiding an entity and reusing it later is cheaper than deleting it,
// which is what a game with many short-lived entities should do.
//
// It panics when i is not an entity, which is a programmer error.
func (e *Engine) Delete(i int) {
	e.Alpha = slices.Delete(e.Alpha, i, i+1)
	e.FrameOffset = slices.Delete(e.FrameOffset, i, i+1)
	e.FrameTime = slices.Delete(e.FrameTime, i, i+1)
	e.ImageColumn = slices.Delete(e.ImageColumn, i, i+1)
	e.ImageIndex = slices.Delete(e.ImageIndex, i, i+1)
	e.ImageRow = slices.Delete(e.ImageRow, i, i+1)
	e.ScreenSpace = slices.Delete(e.ScreenSpace, i, i+1)
	e.SpeedFactor = slices.Delete(e.SpeedFactor, i, i+1)
	e.SpriteHeight = slices.Delete(e.SpriteHeight, i, i+1)
	e.SpriteWidth = slices.Delete(e.SpriteWidth, i, i+1)
	e.State = slices.Delete(e.State, i, i+1)
	e.X = slices.Delete(e.X, i, i+1)
	e.Y = slices.Delete(e.Y, i, i+1)
	e.Z = slices.Delete(e.Z, i, i+1)

	// drawOrder holds indices, not positions, so it is rebuilt in place.
	n := 0
	for _, idx := range e.drawOrder {
		if idx == i {
			continue
		}
		if idx > i {
			idx--
		}
		e.drawOrder[n] = idx
		n++
	}
	e.drawOrder = e.drawOrder[:n]

	e.CamTarget = shiftIndex(e.CamTarget, i)
	e.InputTarget = shiftIndex(e.InputTarget, i)
}

// HasCollision reports whether the hit boxes of entities i and j overlap.
func (e *Engine) HasCollision(i, j int) bool {
	il, it, ir, ib := e.BoundingBox(i)
	jl, jt, jr, jb := e.BoundingBox(j)
	return il < jr && ir > jl && it < jb && ib > jt
}

// Reset removes every entity, keeping the capacity the slices have grown to. A
// game calls it once per scene. It leaves the settings and the camera alone.
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
	e.drawOrder = e.drawOrder[:0]
	e.CamTarget, e.InputTarget = -1, -1
}

// shiftIndex returns where a held index points after entity deleted is gone.
func shiftIndex(idx, deleted int) int {
	switch {
	case idx == deleted:
		return -1
	case idx > deleted:
		return idx - 1
	}
	return idx
}
