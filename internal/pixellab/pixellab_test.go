package pixellab_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"slices"
	"testing"

	"github.com/andygeiss/wisp-engine/internal/pixellab"
)

// The fixtures are real exports, not typed ones: the mage's spritesheet from
// the account, and the probe tileset's metadata, both as PixelLab served them
// on 2026-09-10. A fixture somebody typed only proves the parser agrees with
// a guess about the format.
func read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseLayoutReadsTheExport(t *testing.T) {
	t.Parallel()
	l, err := pixellab.ParseLayout(read(t, "mage.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := l.Spritesheet
	if s.CellSize != (pixellab.Size{Width: 40, Height: 40}) || s.Columns != 8 || len(s.Rows) != 2 {
		t.Errorf("cells %+v, %d columns, %d rows; want 40x40, 8, 2", s.CellSize, s.Columns, len(s.Rows))
	}
	if s.SheetSize != (pixellab.Size{Width: 320, Height: 80}) || s.Pivot != "cell-center" {
		t.Errorf("sheet %+v pivot %q, want 320x80 cell-center", s.SheetSize, s.Pivot)
	}
	rot := s.Rows[0]
	if rot.Type != "rotations" || rot.FrameCount != 8 || len(rot.Directions) != 8 || rot.Directions[0] != "south" || rot.Directions[7] != "south-west" {
		t.Errorf("row 0 = %+v, want the eight rotations from south clockwise", rot)
	}
	anim := s.Rows[1]
	if anim.Type != "animation" || anim.Animation != "Fireball" || anim.Direction != "south-west" || anim.FrameCount != 6 {
		t.Errorf("row 1 = %+v, want Fireball south-west, 6 frames", anim)
	}
	if l.Character.Directions != 8 || l.Crop != nil || l.ExportVersion != "1.0" {
		t.Errorf("directions %d crop %v version %q", l.Character.Directions, l.Crop, l.ExportVersion)
	}
}

func TestParseLayoutRefusesALieAboutTheGrid(t *testing.T) {
	t.Parallel()
	for name, edit := range map[string]func([]byte) []byte{
		"a sheet size that is not the cells": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"width": 320`), []byte(`"width": 321`), 1)
		},
		"a row with more frames than columns": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"frame_count": 6`), []byte(`"frame_count": 9`), 1)
		},
		"not json": func(b []byte) []byte { return b[:10] },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := pixellab.ParseLayout(edit(read(t, "mage.json")))
			if err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestWangReadsTheBoxesNotTheNames(t *testing.T) {
	t.Parallel()
	ts, err := pixellab.ParseTileset(read(t, "meadow.json"))
	if err != nil {
		t.Fatal(err)
	}
	if ts.Data.TileSize != (pixellab.Size{Width: 32, Height: 32}) || ts.Cols() != 4 || ts.Data.TotalTiles != 16 {
		t.Errorf("tile %+v, %d columns, %d tiles", ts.Data.TileSize, ts.Cols(), ts.Data.TotalTiles)
	}
	table, err := ts.Wang()
	if err != nil {
		t.Fatal(err)
	}
	// The sheet's order, measured on the probe: row-major, the corner index
	// of each cell. It is not the index order, which is the whole point.
	sheet := [16]int{13, 10, 4, 12, 6, 8, 0, 1, 11, 3, 2, 5, 15, 14, 9, 7}
	for cell, idx := range sheet {
		if table[idx] != cell {
			t.Errorf("corner index %d is at cell %d, want %d", idx, table[idx], cell)
		}
	}
	if ts.Metadata.TerrainPrompts["lower"] != "deep blue lake water" {
		t.Errorf("lower terrain = %q", ts.Metadata.TerrainPrompts["lower"])
	}
}

func TestWangRefusesWhatItCannotTile(t *testing.T) {
	t.Parallel()
	for name, edit := range map[string]func(*pixellab.Tileset){
		"a box off the grid": func(ts *pixellab.Tileset) {
			ts.Data.Tiles[3].BoundingBox.X++
		},
		"a box of the wrong size": func(ts *pixellab.Tileset) {
			ts.Data.Tiles[3].BoundingBox.Width = 16
		},
		"two tiles with the same corners": func(ts *pixellab.Tileset) {
			ts.Data.Tiles[1].Corners = ts.Data.Tiles[0].Corners
		},
		"a corner that is neither lower nor upper": func(ts *pixellab.Tileset) {
			ts.Data.Tiles[5].Corners["SE"] = "transition"
		},
		"fifteen tiles": func(ts *pixellab.Tileset) {
			ts.Data.Tiles = ts.Data.Tiles[1:]
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts, err := pixellab.ParseTileset(read(t, "meadow.json"))
			if err != nil {
				t.Fatal(err)
			}
			edit(&ts)
			if _, err := ts.Wang(); !errors.Is(err, pixellab.ErrShape) {
				t.Fatalf("err = %v, want ErrShape", err)
			}
		})
	}
	t.Run("not json", func(t *testing.T) {
		t.Parallel()
		if _, err := pixellab.ParseTileset(read(t, "meadow.json")[:20]); err == nil {
			t.Fatal("accepted")
		}
	})
}

func TestCropFindsTheFigure(t *testing.T) {
	t.Parallel()
	f, err := os.Open("testdata/mage.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	cell := pixellab.Size{Width: 40, Height: 40}
	r, ok := pixellab.Crop(img, cell)
	if !ok {
		t.Fatal("an empty sheet")
	}
	// Measured on the export: the figure and its fireball span columns 10 to
	// 29 and rows 7 to 30 of the cell, over every frame.
	if want := (pixellab.Rect{X: 10, Y: 7, Width: 20, Height: 24}); r != want {
		t.Errorf("crop = %+v, want %+v", r, want)
	}

	out := pixellab.CropSheet(img, cell, r)
	if b := out.Bounds(); b.Dx() != 8*20 || b.Dy() != 2*24 {
		t.Errorf("cropped sheet is %dx%d, want 160x48", b.Dx(), b.Dy())
	}
	// The pixel at the crop's origin in cell 0 is the sheet's (10, 7).
	if got, want := out.At(0, 0), img.At(10, 7); !sameColor(got, want) {
		t.Errorf("cropped (0,0) = %v, want the source's (10,7) %v", got, want)
	}
	// And cell 1 of row 1 starts where the source's cell (1,1) plus the crop is.
	if got, want := out.At(20, 24), img.At(40+10, 40+7); !sameColor(got, want) {
		t.Errorf("cropped (20,24) = %v, want the source's (50,47) %v", got, want)
	}
}

func TestCropOfNothingIsFalse(t *testing.T) {
	t.Parallel()
	if _, ok := pixellab.Crop(image.NewRGBA(image.Rect(0, 0, 16, 16)), pixellab.Size{Width: 8, Height: 8}); ok {
		t.Error("an empty sheet had a crop")
	}
}

func sameColor(a, b color.Color) bool {
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

func TestRowOrderSortsAnimationsByName(t *testing.T) {
	t.Parallel()
	var l pixellab.Layout
	l.Spritesheet.Rows = []pixellab.Row{
		{Row: 0, Type: "animation", Animation: "walk", Direction: "east", FrameCount: 8},
		{Row: 1, Type: "animation", Animation: "strike", Direction: "south", FrameCount: 8},
		{Row: 2, Type: "rotations", Directions: []string{"south", "east"}, FrameCount: 2},
		{Row: 3, Type: "animation", Animation: "walk", Direction: "south", FrameCount: 8},
		{Row: 4, Type: "animation", Animation: "idle", Direction: "south", FrameCount: 4},
	}
	got := pixellab.RowOrder(l)
	// The rotations, then idle, strike and walk by name, and walk's south
	// before its east because that is the rotations row's order.
	if want := []int{2, 4, 1, 3, 0}; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	if got := pixellab.RowOrder(pixellab.Layout{}); len(got) != 0 {
		t.Errorf("an empty layout has an order %v", got)
	}
}

func TestReorderRowsMovesWholeRows(t *testing.T) {
	t.Parallel()
	// Three rows of one 2x2 cell, each a different colour.
	img := image.NewRGBA(image.Rect(0, 0, 2, 6))
	colors := []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}}
	for row, c := range colors {
		for y := range 2 {
			for x := range 2 {
				img.Set(x, row*2+y, c)
			}
		}
	}
	out := pixellab.ReorderRows(img, pixellab.Size{Width: 2, Height: 2}, []int{2, 0, 1})
	if b := out.Bounds(); b.Dx() != 2 || b.Dy() != 6 {
		t.Fatalf("reordered sheet is %dx%d", b.Dx(), b.Dy())
	}
	for row, want := range []color.RGBA{colors[2], colors[0], colors[1]} {
		if got := out.RGBAAt(1, row*2+1); got != want {
			t.Errorf("row %d is %v, want %v", row, got, want)
		}
	}
}
