package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures are synthetic where the plumbing is what is tested — a sheet
// of two cells with one opaque pixel each says exactly what a crop has to do
// — and real where the format is: the tileset metadata is the probe's, as
// PixelLab served it, because a typed one would only prove the tool agrees
// with a guess.
const realTileset = "../../internal/pixellab/testdata/meadow.json"

// sheet returns a 2x3 grid of 8x8 cells and the layout that describes it:
// the rotations, then a walk and an idle in PixelLab's order, which is not
// the one the tool writes. Cell (0,0) has one red pixel at (2,3), cell (1,1)
// one blue pixel at (5,6), and cell (0,2) one green pixel at (2,3), so the
// crop is x 2..5, y 3..6 and every row can be told apart.
func sheet(t *testing.T) (pngBytes, layoutBytes []byte) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 24))
	img.Set(2, 3, color.RGBA{255, 0, 0, 255})
	img.Set(8+5, 8+6, color.RGBA{0, 0, 255, 255})
	img.Set(2, 16+3, color.RGBA{0, 255, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	layout := `{
  "character": {"id": "x", "name": "Test", "size": {"width": 8, "height": 8}, "directions": 4},
  "spritesheet": {
    "path": "test.png", "cell_size": {"width": 8, "height": 8}, "sheet_size": {"width": 16, "height": 24},
    "columns": 2, "pivot": "cell-center",
    "rows": [
      {"row": 0, "type": "rotations", "frame_count": 2, "directions": ["south", "east"]},
      {"row": 1, "type": "animation", "frame_count": 1, "animation": "walk", "direction": "south"},
      {"row": 2, "type": "animation", "frame_count": 2, "animation": "idle", "direction": "south"}
    ]
  },
  "export_version": "1.0"
}`
	return buf.Bytes(), []byte(layout)
}

func zipped(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	zw.Close()
	return buf.Bytes()
}

func solidPNG(t *testing.T, w, h int, opaque image.Rectangle) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := opaque.Min.Y; y < opaque.Max.Y; y++ {
		for x := opaque.Min.X; x < opaque.Max.X; x++ {
			img.Set(x, y, color.RGBA{0, 128, 0, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// server is a stand-in for PixelLab's export endpoints, holding one of each
// kind and one character that is still generating.
func server(t *testing.T) *httptest.Server {
	t.Helper()
	sheetPNG, layout := sheet(t)
	meta, err := os.ReadFile(realTileset)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	downloads := 0
	mux.HandleFunc("GET /characters/11111111-1111-1111-1111-111111111111/spritesheet", func(w http.ResponseWriter, r *http.Request) {
		// PixelLab stamps every download with the moment it was made.
		downloads++
		stamped := bytes.Replace(layout, []byte(`"export_version": "1.0"`),
			[]byte(fmt.Sprintf(`"export_version": "1.0", "export_date": "2026-09-10T%02d:00:00"`, downloads)), 1)
		w.Header().Set("Content-Type", "application/zip")
		w.Write(zipped(t, map[string][]byte{"Test.png": sheetPNG, "Test.json": stamped}))
	})
	mux.HandleFunc("GET /characters/22222222-2222-2222-2222-222222222222/spritesheet", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusLocked)
	})
	mux.HandleFunc("GET /tilesets/33333333-3333-3333-3333-333333333333/metadata", func(w http.ResponseWriter, r *http.Request) {
		w.Write(meta)
	})
	mux.HandleFunc("GET /tilesets/33333333-3333-3333-3333-333333333333/image", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("inline") != "true" {
			http.Error(w, "the storage host", http.StatusForbidden)
			return
		}
		w.Write(solidPNG(t, 128, 128, image.Rect(0, 0, 128, 128)))
	})
	mux.HandleFunc("GET /map-objects/44444444-4444-4444-4444-444444444444/download", func(w http.ResponseWriter, r *http.Request) {
		w.Write(solidPNG(t, 32, 32, image.Rect(10, 20, 14, 30)))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestReadManifest(t *testing.T) {
	t.Parallel()
	good := `# the lab's art
character 11111111-1111-1111-1111-111111111111 hero

tileset   33333333-3333-3333-3333-333333333333 meadow
object    44444444-4444-4444-4444-444444444444 tree
`
	assets, err := readManifest(strings.NewReader(good))
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 3 || assets[0] != (asset{"character", "11111111-1111-1111-1111-111111111111", "hero"}) || assets[2].Name != "tree" {
		t.Errorf("assets = %+v", assets)
	}

	for name, bad := range map[string]string{
		"a kind nobody knows":   "font 11111111-1111-1111-1111-111111111111 hero",
		"an id that is not one": "character not-an-id hero",
		"a name with a slash":   "character 11111111-1111-1111-1111-111111111111 ../hero",
		"a name with a capital": "character 11111111-1111-1111-1111-111111111111 Hero",
		"two fields":            "character 11111111-1111-1111-1111-111111111111",
		"the same name twice":   "character 11111111-1111-1111-1111-111111111111 hero\nobject 44444444-4444-4444-4444-444444444444 hero",
		"nothing":               "# only a comment\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := readManifest(strings.NewReader(bad)); err == nil {
				t.Fatalf("accepted %q", bad)
			}
		})
	}
}

func TestRunWritesWhatTheEngineReads(t *testing.T) {
	t.Parallel()
	srv := server(t)
	out := t.TempDir()
	assets := []asset{
		{"character", "11111111-1111-1111-1111-111111111111", "hero"},
		{"tileset", "33333333-3333-3333-3333-333333333333", "meadow"},
		{"object", "44444444-4444-4444-4444-444444444444", "tree"},
	}
	var report bytes.Buffer
	if err := run(srv.Client(), srv.URL, out, assets, &report); err != nil {
		t.Fatalf("run: %v\n%s", err, report.String())
	}

	// The character: the rows sorted — the rotations, then idle before walk
	// — and every cell cut to the union of its opaque pixels, which is x
	// 2..5 and y 3..6, so 4x4 cells and an 8x12 sheet.
	hero := decode(t, filepath.Join(out, "hero.png"))
	if b := hero.Bounds(); b.Dx() != 8 || b.Dy() != 12 {
		t.Errorf("hero.png is %dx%d, want 8x12", b.Dx(), b.Dy())
	}
	if _, _, _, a := hero.At(0, 0).RGBA(); a == 0 {
		t.Error("the red pixel at cell (0,0) 2,3 is not at the cropped sheet's 0,0")
	}
	if _, _, _, a := hero.At(0, 4).RGBA(); a == 0 {
		t.Error("the green pixel of the idle row is not on the cropped sheet's second row")
	}
	if _, _, _, a := hero.At(4+3, 8+3).RGBA(); a == 0 {
		t.Error("the blue pixel of the walk row is not on the cropped sheet's third row")
	}
	var layout struct {
		Spritesheet struct {
			Path      string         `json:"path"`
			CellSize  map[string]int `json:"cell_size"`
			SheetSize map[string]int `json:"sheet_size"`
			Columns   int            `json:"columns"`
			Rows      []struct {
				Row       int    `json:"row"`
				Type      string `json:"type"`
				Animation string `json:"animation"`
			} `json:"rows"`
		} `json:"spritesheet"`
		Crop          map[string]int `json:"crop"`
		ExportVersion string         `json:"export_version"`
	}
	data, err := os.ReadFile(filepath.Join(out, "hero.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &layout); err != nil {
		t.Fatal(err)
	}
	s := layout.Spritesheet
	if s.Path != "hero.png" || s.CellSize["width"] != 4 || s.CellSize["height"] != 4 || s.SheetSize["width"] != 8 || s.SheetSize["height"] != 12 {
		t.Errorf("layout describes %s %v %v, want hero.png 4x4 cells on an 8x12 sheet", s.Path, s.CellSize, s.SheetSize)
	}
	if layout.Crop["x"] != 2 || layout.Crop["y"] != 3 || layout.Crop["width"] != 4 || layout.Crop["height"] != 4 {
		t.Errorf("crop = %v, want 2,3 4x4", layout.Crop)
	}
	if s.Columns != 2 || len(s.Rows) != 3 || layout.ExportVersion != "1.0" {
		t.Errorf("the rest of the layout did not survive: %d columns, %d rows, version %q", s.Columns, len(s.Rows), layout.ExportVersion)
	}
	if bytes.Contains(data, []byte("export_date")) {
		t.Error("the layout kept the export date, which is when it was downloaded and not what it is")
	}
	for i, r := range s.Rows {
		want := []string{"", "idle", "walk"}[i]
		if r.Row != i || r.Animation != want {
			t.Errorf("row %d is numbered %d and is %q, want %q", i, r.Row, r.Animation, want)
		}
	}

	// The tileset: as served, both files.
	meta, _ := os.ReadFile(realTileset)
	if got, _ := os.ReadFile(filepath.Join(out, "meadow.json")); !bytes.Equal(got, meta) {
		t.Error("meadow.json is not the metadata as served")
	}
	if b := decode(t, filepath.Join(out, "meadow.png")).Bounds(); b.Dx() != 128 || b.Dy() != 128 {
		t.Errorf("meadow.png is %dx%d, want 128x128", b.Dx(), b.Dy())
	}

	// The object: cropped to its opaque pixels.
	if b := decode(t, filepath.Join(out, "tree.png")).Bounds(); b.Dx() != 4 || b.Dy() != 10 {
		t.Errorf("tree.png is %dx%d, want 4x10", b.Dx(), b.Dy())
	}

	for _, line := range []string{"character hero: 2 columns x 3 rows of 8x8, cropped to 4x4 at 2,3", "tileset meadow: 16 tiles of 32x32", "object tree: 32x32, cropped to 4x10 at 10,20"} {
		if !strings.Contains(report.String(), line) {
			t.Errorf("report lacks %q:\n%s", line, report.String())
		}
	}

	// A second run changes nothing.
	before := tree(t, out)
	if err := run(srv.Client(), srv.URL, out, assets, &report); err != nil {
		t.Fatal(err)
	}
	after := tree(t, out)
	for name, data := range before {
		if !bytes.Equal(after[name], data) {
			t.Errorf("%s changed on the second run", name)
		}
	}
	if len(after) != len(before) {
		t.Errorf("%d files after the second run, want %d", len(after), len(before))
	}
}

func TestRunReportsWhatIsStillGenerating(t *testing.T) {
	t.Parallel()
	srv := server(t)
	out := t.TempDir()
	assets := []asset{
		{"character", "22222222-2222-2222-2222-222222222222", "hero"},
		{"object", "44444444-4444-4444-4444-444444444444", "tree"},
	}
	var report bytes.Buffer
	err := run(srv.Client(), srv.URL, out, assets, &report)
	if !errors.Is(err, errGenerating) || !strings.Contains(err.Error(), "retry after 30 seconds") {
		t.Fatalf("err = %v, want still generating with the Retry-After", err)
	}
	// The failure did not stop the rest.
	if _, err := os.Stat(filepath.Join(out, "tree.png")); err != nil {
		t.Errorf("the object after the failed character was not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "hero.png")); err == nil {
		t.Error("a sheet was written for a character that is still generating")
	}
}

func TestRunRefusesAnUnknownAsset(t *testing.T) {
	t.Parallel()
	srv := server(t)
	err := run(srv.Client(), srv.URL, t.TempDir(), []asset{{"object", "99999999-9999-9999-9999-999999999999", "ghost"}}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want the 404", err)
	}
}

func decode(t *testing.T, name string) image.Image {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return img
}

func tree(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[e.Name()] = data
	}
	return files
}
