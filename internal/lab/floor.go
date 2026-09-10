package lab

import "github.com/andygeiss/wisp-engine"

// The floor is two Wang layers over the world's grid: the meadow, which is
// water and grass, and the dirt path over it. Both come from PixelLab's
// sixteen-tile top-down tilesets, and both are drawn from a vertex grid — a
// value per corner of the cells, one more than the cells each way — the way
// those tilesets are made to be: a cell's tile is picked by its four
// corners, so any shape painted on the vertices has a seamless edge.

// The world's grid, and the layers under the actors.
const (
	TileSize  = 32.0
	TilesCols = 40
	TilesRows = 24
	WorldW    = TilesCols * TileSize
	WorldH    = TilesRows * TileSize

	// A PixelLab tileset is a 4x4 sheet.
	TilesetCols = 4
	TilesetRows = 4

	// ZFloor is the meadow and ZPath the path over it; both are below
	// every layer an actor stands on, so the floor never draws over one.
	ZFloor = -2
	ZPath  = -1
	// ZPlayer is the players' layer, and ZProps the props', which is the
	// same one: inside a layer the draw order is by bottom edge, so a hero
	// walking behind a tree is drawn behind it.
	ZPlayer = 1
	ZProps  = ZPlayer
)

// WangTable is where PixelLab puts each corner index on its sheet: the
// index NW*8 + NE*4 + SW*2 + SE of a tile, with 1 for the upper terrain, to
// the tile's row-major position on the 4x4 sheet. The sheet is not in
// index order and the docs say to trust only the metadata's boxes, so the
// table is what those boxes said on both tilesets, and a test in this
// package derives it from the committed metadata again.
var WangTable = [16]int{6, 7, 10, 9, 2, 11, 4, 15, 5, 14, 1, 8, 3, 0, 13, 12}

// Wang picks one tile for every cell of a cols by rows grid from vertices,
// which has a value for every corner of every cell — (cols+1) by (rows+1),
// row-major, 0 for the lower terrain and 1 for the upper — through table.
// The result is a Tilemap's Tiles. With skipLower set, a cell whose four
// corners are all the lower terrain is left empty, which is what a layer
// drawn over another wants: it shows only where it differs.
func Wang(vertices []uint8, cols, rows int, table [16]int, skipLower bool) []int {
	tiles := make([]int, cols*rows)
	w := cols + 1
	for r := range rows {
		for c := range cols {
			idx := int(vertices[r*w+c]&1)<<3 | int(vertices[r*w+c+1]&1)<<2 |
				int(vertices[(r+1)*w+c]&1)<<1 | int(vertices[(r+1)*w+c+1]&1)
			if skipLower && idx == 0 {
				tiles[r*cols+c] = -1
				continue
			}
			tiles[r*cols+c] = table[idx]
		}
	}
	return tiles
}

// LakeVertices is the meadow's vertex grid: 1 where there is grass, 0 where
// the lake is. The lake is two overlapping ellipses up and left of the
// middle, because the middle is where a player spawns.
func LakeVertices() []uint8 {
	v := make([]uint8, (TilesCols+1)*(TilesRows+1))
	for y := range TilesRows + 1 {
		for x := range TilesCols + 1 {
			if !inEllipse(x, y, 12, 9, 6, 4) && !inEllipse(x, y, 16, 7, 3, 2.5) {
				v[y*(TilesCols+1)+x] = 1
			}
		}
	}
	return v
}

// PathVertices is the path's vertex grid: 1 where there is dirt. A band
// runs across below the lake and a branch goes up from it on the right;
// neither reaches the lake's shore, because a path tile is drawn over the
// meadow's and would cover it.
func PathVertices() []uint8 {
	v := make([]uint8, (TilesCols+1)*(TilesRows+1))
	for y := range TilesRows + 1 {
		for x := range TilesCols + 1 {
			band := y >= 19 && y <= 21 && x >= 2 && x <= 38
			branch := x >= 33 && x <= 35 && y >= 3 && y <= 21
			if band || branch {
				v[y*(TilesCols+1)+x] = 1
			}
		}
	}
	return v
}

// inEllipse reports whether vertex (x, y) is inside the ellipse centred on
// (cx, cy) with the half-widths rx and ry.
func inEllipse(x, y int, cx, cy, rx, ry float64) bool {
	dx, dy := (float64(x)-cx)/rx, (float64(y)-cy)/ry
	return dx*dx+dy*dy <= 1
}

// BuildFloor adds the floor: the meadow on [ZFloor], every cell, and the
// path on [ZPath] where there is any. The floor never moves and collides
// with nothing, so the server never has one — each client builds its own
// and it never crosses the wire.
func BuildFloor(e *wisp.Engine) {
	layer := func(image int, tiles []int, z int) {
		e.AddTilemap(wisp.Tilemap{
			Cols: TilesCols, Height: TileSize, Image: image, Rows: TilesRows,
			Tiles: tiles, TilesetCols: TilesetCols, TilesetRows: TilesetRows, Width: TileSize,
			Z: z,
		})
	}
	layer(ImageMeadow, Wang(LakeVertices(), TilesCols, TilesRows, WangTable, false), ZFloor)
	layer(ImagePath, Wang(PathVertices(), TilesCols, TilesRows, WangTable, true), ZPath)
}
