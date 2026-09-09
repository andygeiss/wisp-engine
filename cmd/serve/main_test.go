package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTree builds a web tree in a temporary directory. The tests never touch
// the repository's own, so they pass on a fresh clone that has never run
// `make wasm`.
func newTree(t *testing.T, wasm string) fs.FS {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"templates/index.html": `<html data-version="{{version}}"><script src="/static/lab.wasm?v={{version}}"></script></html>`,
		"static/lab.wasm":      wasm,
		"static/css/app.css":   "canvas { display: block; }",
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root.FS()
}

// get runs one request through the whole chain, in process. Nothing here
// listens on a port, so no test can reach a stranger's server.
func get(t *testing.T, tree fs.FS, path string) *http.Response {
	t.Helper()
	if err := registerTypes(); err != nil {
		t.Fatal(err)
	}
	version, err := treeVersion(tree)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	routes(tree, version).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w.Result()
}

func TestSecureHeaders(t *testing.T) {
	t.Parallel()
	res := get(t, newTree(t, "\x00asm"), "/")

	want := map[string]string{
		"Content-Security-Policy": csp,
		"Referrer-Policy":         "same-origin",
		"X-Content-Type-Options":  "nosniff",
	}
	for name, value := range want {
		if got := res.Header.Get(name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	// WebAssembly needs one addition to the baseline policy, and only one.
	if !strings.Contains(csp, "'wasm-unsafe-eval'") {
		t.Error("the policy has no 'wasm-unsafe-eval', so no browser will compile the module")
	}
	for _, banned := range []string{"'unsafe-eval'", "'unsafe-inline'"} {
		// 'wasm-unsafe-eval' contains 'unsafe-eval' as a substring, so the
		// check has to look at the tokens rather than the whole string.
		for token := range strings.FieldsSeq(csp) {
			if strings.TrimSuffix(token, ";") == banned {
				t.Errorf("the policy carries %s", banned)
			}
		}
	}
}

// TestWasmContentType guards the one mistake that breaks the lab in silence.
func TestWasmContentType(t *testing.T) {
	t.Parallel()
	res := get(t, newTree(t, "\x00asm"), "/static/lab.wasm")

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "application/wasm" {
		t.Errorf("Content-Type = %q, want application/wasm", got)
	}
}

func TestCaching(t *testing.T) {
	t.Parallel()
	tree := newTree(t, "\x00asm")

	if got := get(t, tree, "/static/lab.wasm").Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("static Cache-Control = %q, want it immutable", got)
	}
	if got := get(t, tree, "/").Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("page Cache-Control = %q, want no-store", got)
	}
}

func TestPageCarriesTheVersion(t *testing.T) {
	t.Parallel()
	tree := newTree(t, "\x00asm")
	version, err := treeVersion(tree)
	if err != nil {
		t.Fatal(err)
	}

	res := get(t, tree, "/")
	body := make([]byte, 4096)
	n, _ := res.Body.Read(body)
	page := string(body[:n])

	if strings.Contains(page, "{{version}}") {
		t.Errorf("the page still has a placeholder in it:\n%s", page)
	}
	if !strings.Contains(page, `data-version="`+version+`"`) {
		t.Errorf("the page does not carry version %q:\n%s", version, page)
	}
}

// TestVersionTracksTheAssets is the promise the year-long cache rests on. The
// binary's own identity says nothing here, because the assets are on disk.
func TestVersionTracksTheAssets(t *testing.T) {
	t.Parallel()
	one := newTree(t, "\x00asm one")
	two := newTree(t, "\x00asm two")

	a, err := treeVersion(one)
	if err != nil {
		t.Fatal(err)
	}
	again, err := treeVersion(one)
	if err != nil {
		t.Fatal(err)
	}
	b, err := treeVersion(two)
	if err != nil {
		t.Fatal(err)
	}

	if a != again {
		t.Errorf("the same tree gave %q then %q: a restart would break every cache", a, again)
	}
	if a == b {
		t.Errorf("two different modules share the version %q, so a rebuild would serve the old one", a)
	}
}

func TestStaticDirectoryIsNotListed(t *testing.T) {
	t.Parallel()
	if got := get(t, newTree(t, "\x00asm"), "/static/").StatusCode; got != http.StatusNotFound {
		t.Errorf("status = %d, want 404: the directory listed its files", got)
	}
}

func TestTraversalIsRefused(t *testing.T) {
	t.Parallel()
	tree := newTree(t, "\x00asm")
	for _, path := range []string{
		"/static/../templates/index.html",
		"/static/..%2ftemplates%2findex.html",
		"/static/../../etc/passwd",
	} {
		if got := get(t, tree, path).StatusCode; got == http.StatusOK {
			t.Errorf("%s answered 200, want it refused", path)
		}
	}
}
