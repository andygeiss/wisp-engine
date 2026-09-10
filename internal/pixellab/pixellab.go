// Package pixellab reads what PixelLab exports: the layout JSON beside a
// character's spritesheet, and the metadata JSON beside a tileset's sheet.
//
// It is for the host — cmd/art, which downloads and reshapes the art, and the
// tests that check the lab's hand-written constants against the committed
// files. It uses encoding/json and image/png, which is why it is never
// imported by anything TinyGo builds: the browser module reads no JSON at
// all, and the sheet a game attaches is a handful of constants a test here
// keeps honest.
package pixellab

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"sort"
)

// Size is a width and a height in pixels, the way PixelLab spells one.
type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Rect is a rectangle on an image, the way PixelLab spells a bounding box.
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Layout is the JSON beside a spritesheet PNG: one uniform grid, row 0 the
// rotations in PixelLab's standard order, then one row per animation and
// direction. It reads the fields the engine needs and keeps the rest of the
// file as it was.
type Layout struct {
	Character struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Size       Size   `json:"size"`
		Directions int    `json:"directions"`
	} `json:"character"`
	Spritesheet struct {
		Path      string `json:"path"`
		CellSize  Size   `json:"cell_size"`
		SheetSize Size   `json:"sheet_size"`
		Columns   int    `json:"columns"`
		Pivot     string `json:"pivot"`
		Rows      []Row  `json:"rows"`
	} `json:"spritesheet"`
	// Crop is what cmd/art cut from every cell, in the pixels of the cell
	// PixelLab exported. A file straight from PixelLab has none.
	Crop          *Rect  `json:"crop,omitempty"`
	ExportVersion string `json:"export_version"`
}

// Row is one row of the sheet: the rotations, or one animation in one
// direction.
type Row struct {
	Row        int    `json:"row"`
	Type       string `json:"type"`
	FrameCount int    `json:"frame_count"`
	// Directions is set on the rotations row: which direction each column
	// shows.
	Directions []string `json:"directions,omitempty"`
	// Animation and Direction are set on an animation row.
	Animation string `json:"animation,omitempty"`
	Direction string `json:"direction,omitempty"`
}

// Tileset is the metadata JSON beside a tileset sheet, as far as the engine
// needs it: the tile size, and every tile's corners and its box on the sheet.
type Tileset struct {
	ID     string `json:"id"`
	Layout struct {
		Type      string `json:"type"`
		GridSize  Size   `json:"grid_size"`
		TileCount int    `json:"tile_count"`
	} `json:"layout"`
	TileSize       Size    `json:"tile_size"`
	TransitionSize float64 `json:"transition_size"`
	Data           struct {
		TotalTiles int    `json:"total_tiles"`
		TileSize   Size   `json:"tile_size"`
		Tiles      []Tile `json:"tiles"`
	} `json:"tileset_data"`
	Metadata struct {
		TerrainPrompts map[string]string `json:"terrain_prompts"`
	} `json:"metadata"`
}

// Tile is one tile: its corners, lower or upper, and where it is on the sheet.
type Tile struct {
	Name        string            `json:"name"`
	Corners     map[string]string `json:"corners"`
	BoundingBox Rect              `json:"bounding_box"`
}

// ErrShape means a file parsed but does not describe what it claims: a sheet
// whose size is not its cells, a tileset that is not sixteen tiles on a grid.
var ErrShape = errors.New("pixellab: export has an unexpected shape")

// ParseLayout reads a layout JSON and checks that the sheet it describes is
// the grid it claims: the sheet size is the cell size times the columns and
// the rows, and no row holds more frames than there are columns.
func ParseLayout(data []byte) (Layout, error) {
	var l Layout
	if err := json.Unmarshal(data, &l); err != nil {
		return Layout{}, fmt.Errorf("pixellab: layout: %w", err)
	}
	s := &l.Spritesheet
	if s.Columns <= 0 || len(s.Rows) == 0 || s.CellSize.Width <= 0 || s.CellSize.Height <= 0 {
		return Layout{}, fmt.Errorf("%w: no grid", ErrShape)
	}
	if s.SheetSize.Width != s.Columns*s.CellSize.Width || s.SheetSize.Height != len(s.Rows)*s.CellSize.Height {
		return Layout{}, fmt.Errorf("%w: sheet %dx%d is not %d columns and %d rows of %dx%d cells",
			ErrShape, s.SheetSize.Width, s.SheetSize.Height, s.Columns, len(s.Rows), s.CellSize.Width, s.CellSize.Height)
	}
	for i, r := range s.Rows {
		if r.Row != i || r.FrameCount <= 0 || r.FrameCount > s.Columns {
			return Layout{}, fmt.Errorf("%w: row %d is numbered %d with %d frames of %d columns", ErrShape, i, r.Row, r.FrameCount, s.Columns)
		}
	}
	return l, nil
}

// ParseTileset reads a tileset metadata JSON and checks that it is a
// sixteen-tile Wang set whose tiles sit on a grid of its tile size.
func ParseTileset(data []byte) (Tileset, error) {
	var t Tileset
	if err := json.Unmarshal(data, &t); err != nil {
		return Tileset{}, fmt.Errorf("pixellab: tileset: %w", err)
	}
	if _, err := t.Wang(); err != nil {
		return Tileset{}, err
	}
	return t, nil
}

// Cols is how many tiles wide the sheet is.
func (t Tileset) Cols() int {
	if t.Layout.GridSize.Width > 0 {
		return t.Layout.GridSize.Width
	}
	return 4
}

// Wang returns, for every corner index NW*8 + NE*4 + SW*2 + SE, the index of
// the tile on the sheet, row-major — what a Tilemap's Tiles slice holds. The
// sheet is not in that order, and the docs say to trust only the boxes, so
// the boxes are what this reads.
//
// It refuses a set that is not sixteen tiles of lower and upper corners, one
// for every index, each box the tile size and on the grid.
func (t Tileset) Wang() ([16]int, error) {
	var table [16]int
	var seen [16]bool
	size := t.Data.TileSize
	if size.Width <= 0 || size.Height <= 0 {
		return table, fmt.Errorf("%w: no tile size", ErrShape)
	}
	if len(t.Data.Tiles) != 16 {
		return table, fmt.Errorf("%w: %d tiles, want 16", ErrShape, len(t.Data.Tiles))
	}
	cols := t.Cols()
	for _, tile := range t.Data.Tiles {
		idx := 0
		for bit, corner := range map[int]string{8: "NW", 4: "NE", 2: "SW", 1: "SE"} {
			switch tile.Corners[corner] {
			case "upper":
				idx |= bit
			case "lower":
			default:
				return table, fmt.Errorf("%w: tile %q corner %s is %q", ErrShape, tile.Name, corner, tile.Corners[corner])
			}
		}
		b := tile.BoundingBox
		if b.Width != size.Width || b.Height != size.Height || b.X%size.Width != 0 || b.Y%size.Height != 0 {
			return table, fmt.Errorf("%w: tile %q box %+v is off the %dx%d grid", ErrShape, tile.Name, b, size.Width, size.Height)
		}
		if seen[idx] {
			return table, fmt.Errorf("%w: two tiles with corners %d", ErrShape, idx)
		}
		seen[idx] = true
		table[idx] = b.Y/size.Height*cols + b.X/size.Width
	}
	return table, nil
}

// RowOrder returns the rows of a layout in the order cmd/art writes them:
// the rotations first, then every animation in alphabetical order of its
// name, its directions in the order the rotations row lists them. PixelLab's
// own export orders the animations by their IDs, which is no order a game
// can depend on: a regenerated walk would move every row under it. The
// result is a permutation — result[i] is the index in Rows of the row that
// goes in position i.
func RowOrder(l Layout) []int {
	rows := l.Spritesheet.Rows
	dirs := map[string]int{}
	for _, r := range rows {
		if r.Type == "rotations" {
			for i, d := range r.Directions {
				dirs[d] = i
			}
		}
	}
	order := make([]int, len(rows))
	for i := range order {
		order[i] = i
	}
	rank := func(i int) (int, string, int) {
		r := rows[i]
		if r.Type == "rotations" {
			return 0, "", 0
		}
		d, ok := dirs[r.Direction]
		if !ok {
			d = len(dirs)
		}
		return 1, r.Animation, d
	}
	sort.SliceStable(order, func(a, b int) bool {
		ta, na, da := rank(order[a])
		tb, nb, db := rank(order[b])
		if ta != tb {
			return ta < tb
		}
		if na != nb {
			return na < nb
		}
		return da < db
	})
	return order
}

// ReorderRows returns a copy of a uniform-grid sheet with its rows in the
// given order: row i of the result is row order[i] of img.
func ReorderRows(img image.Image, cell Size, order []int) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), len(order)*cell.Height))
	for i, from := range order {
		src := image.Pt(b.Min.X, b.Min.Y+from*cell.Height)
		dst := image.Rect(0, i*cell.Height, b.Dx(), (i+1)*cell.Height)
		draw.Draw(out, dst, img, src, draw.Src)
	}
	return out
}

// Crop returns the smallest rectangle, in the pixels of one cell, that holds
// every opaque pixel of every cell of a uniform-grid sheet, and false when
// nothing on the sheet is opaque.
//
// It is one rectangle for the whole sheet rather than one per frame, so that
// the frames keep their places relative to each other and an animation does
// not jitter. What it cuts is the padding PixelLab leaves around a figure —
// about forty per cent of the cell — which the engine would otherwise take
// for the sprite: the hit box is the sprite's rectangle less a margin, and
// the draw order is its bottom edge.
func Crop(img image.Image, cell Size) (Rect, bool) {
	b := img.Bounds()
	if cell.Width <= 0 || cell.Height <= 0 {
		return Rect{}, false
	}
	cols, rows := b.Dx()/cell.Width, b.Dy()/cell.Height
	minX, minY, maxX, maxY := cell.Width, cell.Height, -1, -1
	for r := range rows {
		for c := range cols {
			for y := range cell.Height {
				for x := range cell.Width {
					if _, _, _, a := img.At(b.Min.X+c*cell.Width+x, b.Min.Y+r*cell.Height+y).RGBA(); a == 0 {
						continue
					}
					minX, minY = min(minX, x), min(minY, y)
					maxX, maxY = max(maxX, x), max(maxY, y)
				}
			}
		}
	}
	if maxX < 0 {
		return Rect{}, false
	}
	return Rect{X: minX, Y: minY, Width: maxX - minX + 1, Height: maxY - minY + 1}, true
}

// CropSheet returns a copy of a uniform-grid sheet with every cell cut to r,
// so the new cell is r's size and the grid is the same columns and rows.
func CropSheet(img image.Image, cell Size, r Rect) *image.RGBA {
	b := img.Bounds()
	cols, rows := b.Dx()/cell.Width, b.Dy()/cell.Height
	out := image.NewRGBA(image.Rect(0, 0, cols*r.Width, rows*r.Height))
	for row := range rows {
		for col := range cols {
			src := image.Pt(b.Min.X+col*cell.Width+r.X, b.Min.Y+row*cell.Height+r.Y)
			dst := image.Rect(col*r.Width, row*r.Height, (col+1)*r.Width, (row+1)*r.Height)
			draw.Draw(out, dst, img, src, draw.Src)
		}
	}
	return out
}
