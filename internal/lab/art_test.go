package lab_test

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/andygeiss/wisp-engine"
	"github.com/andygeiss/wisp-engine/internal/lab"
	"github.com/andygeiss/wisp-engine/internal/pixellab"
)

// The committed art, as cmd/art wrote it. These tests read the real files
// rather than fixtures: a constant in art.go is a claim about a file, and
// the file is what checks it.
const artDir = "../../web/static/img/"

func read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(artDir + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func pngSize(t *testing.T, name string) (w, h int) {
	t.Helper()
	f, err := os.Open(artDir + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return c.Width, c.Height
}

// checkSheet holds a sheet built by art.go against the layout JSON and the
// PNG it describes: the cell, the columns, the row count, and for every tag
// the row's animation, direction and frame count.
func checkSheet(t *testing.T, name string, s wisp.Sheet, w, h, anims int) {
	t.Helper()
	l, err := pixellab.ParseLayout(read(t, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	ss := l.Spritesheet
	if ss.CellSize.Width != w || ss.CellSize.Height != h {
		t.Errorf("%s: the cell is %dx%d, art.go says %dx%d", name, ss.CellSize.Width, ss.CellSize.Height, w, h)
	}
	if ss.Columns != lab.Columns {
		t.Errorf("%s: %d columns, art.go says %d", name, ss.Columns, lab.Columns)
	}
	if want := 1 + lab.NumDirections*(anims-1); len(ss.Rows) != want {
		t.Errorf("%s: %d rows, art.go says %d", name, len(ss.Rows), want)
	}
	if pw, ph := pngSize(t, name+".png"); pw != ss.SheetSize.Width || ph != ss.SheetSize.Height {
		t.Errorf("%s.png is %dx%d, its layout says %dx%d", name, pw, ph, ss.SheetSize.Width, ss.SheetSize.Height)
	}
	if float64(ss.SheetSize.Width) != s.W || float64(ss.SheetSize.Height) != s.H {
		t.Errorf("%s: the Sheet is %vx%v, the layout %dx%d", name, s.W, s.H, ss.SheetSize.Width, ss.SheetSize.Height)
	}
	for i, tag := range s.Tags {
		row := tag.From / lab.Columns
		if row >= len(ss.Rows) {
			t.Errorf("%s: tag %q is on row %d, past the %d the layout has", name, tag.Name, row, len(ss.Rows))
			continue
		}
		r := ss.Rows[row]
		anim, dir, _ := strings.Cut(tag.Name, "-")
		if i < lab.NumDirections {
			// A face tag: one cell of the rotations row, the column its
			// direction sits in.
			if r.Type != "rotations" || tag.From != tag.To || tag.From%lab.Columns >= len(r.Directions) || r.Directions[tag.From%lab.Columns] != dir {
				t.Errorf("%s: tag %q points at row %+v cell %d", name, tag.Name, r, tag.From%lab.Columns)
			}
			continue
		}
		if r.Type != "animation" || r.Animation != anim || r.Direction != dir {
			t.Errorf("%s: tag %q points at row %d, which is %s %s %s", name, tag.Name, row, r.Type, r.Animation, r.Direction)
		}
		if frames := tag.To - tag.From + 1; frames != r.FrameCount || tag.From%lab.Columns != 0 {
			t.Errorf("%s: tag %q is %d frames from column %d, the row has %d", name, tag.Name, frames, tag.From%lab.Columns, r.FrameCount)
		}
	}
}

func TestTheHeroSheetIsTheExport(t *testing.T) {
	t.Parallel()
	checkSheet(t, "hero", lab.HeroSheet(), lab.HeroW, lab.HeroH, lab.NumHeroAnims)
}

func TestTheFoxSheetIsTheExport(t *testing.T) {
	t.Parallel()
	checkSheet(t, "fox", lab.FoxSheet(), lab.FoxW, lab.FoxH, lab.NumFoxAnims)
}

func TestTheTilesetsAreTheMetadata(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"meadow", "path"} {
		ts, err := pixellab.ParseTileset(read(t, name+".json"))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		table, err := ts.Wang()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if table != lab.WangTable {
			t.Errorf("%s: the metadata's table is %v, floor.go says %v", name, table, lab.WangTable)
		}
		if ts.Data.TileSize.Width != lab.TileSize || ts.Data.TileSize.Height != lab.TileSize || ts.Cols() != lab.TilesetCols {
			t.Errorf("%s: tiles of %+v on %d columns, floor.go says %v on %d", name, ts.Data.TileSize, ts.Cols(), lab.TileSize, lab.TilesetCols)
		}
		if w, h := pngSize(t, name+".png"); w != lab.TilesetCols*lab.TileSize || h != lab.TilesetRows*lab.TileSize {
			t.Errorf("%s.png is %dx%d, want %d columns and %d rows of %v", name, w, h, lab.TilesetCols, lab.TilesetRows, lab.TileSize)
		}
	}
}

func TestThePropsAreTheirFiles(t *testing.T) {
	t.Parallel()
	for _, p := range lab.Props {
		file := strings.TrimPrefix(lab.Images[p.Image], "/static/img/")
		if w, h := pngSize(t, file); float64(w) != p.W || float64(h) != p.H {
			t.Errorf("%s is %dx%d, art.go says %vx%v", file, w, h, p.W, p.H)
		}
	}
}

func TestEveryImageIsAFile(t *testing.T) {
	t.Parallel()
	for i, path := range lab.Images {
		if !strings.HasPrefix(path, "/static/img/") {
			t.Errorf("image %d is %q, not under the static tree", i, path)
		}
		pngSize(t, strings.TrimPrefix(path, "/static/img/"))
	}
}
