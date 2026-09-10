// The art: what PixelLab generated, downloaded by ID and reshaped for the
// engine. "make art" runs it.
//
// assets/pixellab.txt names every asset the lab draws, one a line: a kind, a
// PixelLab ID and the name to save it under. For each line this fetches the
// export the ID keys — no token, that is how PixelLab serves them — and writes
// what the engine reads into web/static/img:
//
//   - a character's spritesheet, one uniform grid, cropped to the figure, and
//     its layout JSON rewritten to describe the cropped file;
//   - a tileset's sheet and metadata, byte for byte as served;
//   - a map object's PNG, cropped to the object.
//
// The output is a function of the account's assets and nothing else, so
// running it twice changes nothing, and git says when an asset changed under
// a name. An asset still generating is reported and skipped, and the run
// fails at the end so a half-fetched tree cannot pass for a whole one.
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/andygeiss/wisp-engine/internal/pixellab"
)

// The defaults: where the manifest is, where the art goes, and where PixelLab
// serves it from.
const (
	defaultManifest = "assets/pixellab.txt"
	defaultOut      = "web/static/img"
	defaultAPI      = "https://api.pixellab.ai/mcp"
)

// The kinds a manifest line may name.
const (
	kindCharacter = "character"
	kindObject    = "object"
	kindTileset   = "tileset"
)

// asset is one manifest line.
type asset struct {
	Kind, ID, Name string
}

func main() {
	manifest := flag.String("manifest", defaultManifest, "the assets to fetch, one a line: kind, PixelLab ID, name")
	out := flag.String("out", defaultOut, "the directory the art is written to")
	api := flag.String("api", defaultAPI, "where PixelLab serves the exports")
	flag.Parse()

	f, err := os.Open(*manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	assets, err := readManifest(f)
	f.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	if err := run(client, *api, *out, assets, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// readManifest parses the manifest: blank lines and lines starting with #
// are skipped, and every other line is a kind, an ID and a name separated by
// spaces. A name is what the files are called, so it has to be one that a
// URL and a Go identifier can both carry.
func readManifest(r io.Reader) ([]asset, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var assets []asset
	seen := map[string]bool{}
	for n, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("manifest line %d: want kind, id and name, got %q", n+1, line)
		}
		a := asset{Kind: fields[0], ID: fields[1], Name: fields[2]}
		switch a.Kind {
		case kindCharacter, kindObject, kindTileset:
		default:
			return nil, fmt.Errorf("manifest line %d: unknown kind %q", n+1, a.Kind)
		}
		if !isUUID(a.ID) {
			return nil, fmt.Errorf("manifest line %d: %q is not a PixelLab ID", n+1, a.ID)
		}
		if !isName(a.Name) {
			return nil, fmt.Errorf("manifest line %d: %q is not a name: lower-case letters, digits, - and _ only", n+1, a.Name)
		}
		if seen[a.Name] {
			return nil, fmt.Errorf("manifest line %d: the name %q is taken", n+1, a.Name)
		}
		seen[a.Name] = true
		assets = append(assets, a)
	}
	if len(assets) == 0 {
		return nil, errors.New("manifest names no assets")
	}
	return assets, nil
}

// isUUID reports whether s is shaped like a PixelLab ID: 36 characters of
// hex digits and dashes in the usual places.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return false
			}
		}
	}
	return true
}

// isName reports whether s may name a file here.
func isName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// run fetches every asset and writes it under out, reporting each on w. It
// keeps going past an asset that fails, so one still generating does not
// hide the state of the rest, and returns every failure at the end.
func run(client *http.Client, api, out string, assets []asset, w io.Writer) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	var errs []error
	for _, a := range assets {
		var report string
		var err error
		switch a.Kind {
		case kindCharacter:
			report, err = fetchCharacter(client, api, out, a)
		case kindTileset:
			report, err = fetchTileset(client, api, out, a)
		case kindObject:
			report, err = fetchObject(client, api, out, a)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s %s: %w", a.Kind, a.Name, err))
			fmt.Fprintf(w, "%s %s: %v\n", a.Kind, a.Name, err)
			continue
		}
		fmt.Fprintf(w, "%s %s: %s\n", a.Kind, a.Name, report)
	}
	return errors.Join(errs...)
}

// errGenerating is what a 423 means: the asset exists and is not ready.
var errGenerating = errors.New("still generating")

// get fetches url and returns the body of a 200. A 423 is [errGenerating]
// with PixelLab's Retry-After; anything else is its status.
func get(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// PixelLab's storage refuses some default agents; name this one.
	req.Header.Set("User-Agent", "wisp-engine/art")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	case http.StatusLocked:
		return nil, fmt.Errorf("%w, retry after %s seconds", errGenerating, resp.Header.Get("Retry-After"))
	default:
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
}

// fetchCharacter downloads a character's spritesheet export — a zip holding
// one PNG and one JSON — crops every cell to the figure, and writes the
// cropped sheet and a layout that describes it.
func fetchCharacter(client *http.Client, api, out string, a asset) (string, error) {
	data, err := get(client, api+"/characters/"+a.ID+"/spritesheet")
	if err != nil {
		return "", err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("the export is not a zip: %w", err)
	}
	var sheetPNG, layoutJSON []byte
	for _, f := range zr.File {
		switch path.Ext(f.Name) {
		case ".png":
			sheetPNG, err = readZipped(f)
		case ".json":
			layoutJSON, err = readZipped(f)
		}
		if err != nil {
			return "", err
		}
	}
	if sheetPNG == nil || layoutJSON == nil {
		return "", errors.New("the export holds no PNG and JSON pair")
	}
	layout, err := pixellab.ParseLayout(layoutJSON)
	if err != nil {
		return "", err
	}
	img, err := png.Decode(bytes.NewReader(sheetPNG))
	if err != nil {
		return "", fmt.Errorf("the sheet: %w", err)
	}
	s := layout.Spritesheet
	if b := img.Bounds(); b.Dx() != s.SheetSize.Width || b.Dy() != s.SheetSize.Height {
		return "", fmt.Errorf("%w: the sheet is %dx%d and the layout says %dx%d",
			pixellab.ErrShape, b.Dx(), b.Dy(), s.SheetSize.Width, s.SheetSize.Height)
	}
	// The rows into an order a game can count on, then every cell cut to
	// the figure.
	order := pixellab.RowOrder(layout)
	crop, ok := pixellab.Crop(img, s.CellSize)
	if !ok {
		return "", errors.New("the sheet is empty")
	}
	cropped := pixellab.CropSheet(pixellab.ReorderRows(img, s.CellSize, order), s.CellSize, crop)
	cell := pixellab.Size{Width: crop.Width, Height: crop.Height}
	sheet := pixellab.Size{Width: cropped.Bounds().Dx(), Height: cropped.Bounds().Dy()}
	rewritten, err := rewriteLayout(layoutJSON, a.Name+".png", cell, sheet, crop, order)
	if err != nil {
		return "", err
	}
	if err := writePNG(filepath.Join(out, a.Name+".png"), cropped); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(out, a.Name+".json"), rewritten, 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d columns x %d rows of %dx%d, cropped to %dx%d at %d,%d",
		s.Columns, len(s.Rows), s.CellSize.Width, s.CellSize.Height, cell.Width, cell.Height, crop.X, crop.Y), nil
}

// readZipped returns the whole of one file in a zip.
func readZipped(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 64<<20))
}

// rewriteLayout returns the layout JSON with the sheet's path, cell size and
// sheet size replaced by the cropped file's, the rows in the order the sheet
// was written in and numbered so, the crop recorded, and the export date
// dropped — so the JSON beside the PNG describes the PNG beside it and
// nothing else. Everything else in the file stays as PixelLab wrote it.
func rewriteLayout(data []byte, file string, cell, sheet pixellab.Size, crop pixellab.Rect, order []int) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	s, ok := m["spritesheet"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: no spritesheet object", pixellab.ErrShape)
	}
	rows, ok := s["rows"].([]any)
	if !ok || len(rows) != len(order) {
		return nil, fmt.Errorf("%w: %d rows to order %d", pixellab.ErrShape, len(rows), len(order))
	}
	sorted := make([]any, len(order))
	for i, from := range order {
		row, ok := rows[from].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: row %d is not an object", pixellab.ErrShape, from)
		}
		row["row"] = i
		sorted[i] = row
	}
	s["rows"] = sorted
	s["path"] = file
	s["cell_size"] = cell
	s["sheet_size"] = sheet
	m["crop"] = crop
	// The export date is when the file was downloaded, not what it is, and
	// it is what would make a second run differ from the first.
	delete(m, "export_date")
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// fetchTileset downloads a tileset's sheet and metadata and writes both as
// served. The metadata is parsed first, so a set the engine cannot tile is
// refused here rather than found in a browser.
func fetchTileset(client *http.Client, api, out string, a asset) (string, error) {
	meta, err := get(client, api+"/tilesets/"+a.ID+"/metadata")
	if err != nil {
		return "", err
	}
	ts, err := pixellab.ParseTileset(meta)
	if err != nil {
		return "", err
	}
	// ?inline=true serves the PNG from PixelLab's own host rather than its
	// storage, which is the same file behind fewer refusals.
	sheet, err := get(client, api+"/tilesets/"+a.ID+"/image?inline=true")
	if err != nil {
		return "", err
	}
	img, err := png.Decode(bytes.NewReader(sheet))
	if err != nil {
		return "", fmt.Errorf("the sheet: %w", err)
	}
	size := ts.Data.TileSize
	if b := img.Bounds(); b.Dx() != ts.Cols()*size.Width || b.Dy()*ts.Cols() != len(ts.Data.Tiles)*size.Height {
		return "", fmt.Errorf("%w: the sheet is %dx%d for %d tiles of %dx%d",
			pixellab.ErrShape, b.Dx(), b.Dy(), len(ts.Data.Tiles), size.Width, size.Height)
	}
	if err := os.WriteFile(filepath.Join(out, a.Name+".png"), sheet, 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(out, a.Name+".json"), meta, 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d tiles of %dx%d: %s to %s", len(ts.Data.Tiles), size.Width, size.Height,
		ts.Metadata.TerrainPrompts["lower"], ts.Metadata.TerrainPrompts["upper"]), nil
}

// fetchObject downloads a map object's PNG and writes it cropped to the
// object, so its rectangle is the object and not the canvas it was drawn on.
func fetchObject(client *http.Client, api, out string, a asset) (string, error) {
	data, err := get(client, api+"/map-objects/"+a.ID+"/download")
	if err != nil {
		return "", err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("the object: %w", err)
	}
	b := img.Bounds()
	canvas := pixellab.Size{Width: b.Dx(), Height: b.Dy()}
	crop, ok := pixellab.Crop(img, canvas)
	if !ok {
		return "", errors.New("the object is empty")
	}
	cropped := pixellab.CropSheet(img, canvas, crop)
	if err := writePNG(filepath.Join(out, a.Name+".png"), cropped); err != nil {
		return "", err
	}
	return fmt.Sprintf("%dx%d, cropped to %dx%d at %d,%d", canvas.Width, canvas.Height, crop.Width, crop.Height, crop.X, crop.Y), nil
}

// writePNG encodes img to name. The standard encoder is deterministic, which
// is what lets a second run change nothing.
func writePNG(name string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(name, buf.Bytes(), 0o644)
}
