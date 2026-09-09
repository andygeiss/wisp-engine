// Command serve hosts the lab: one page and the web tree, under the same
// security headers and the same cache contract a deployed game gets.
//
// It is a development tool. It reads from disk, so editing the stylesheet and
// reloading is the whole loop, and it is never deployed — which is why it has
// no ops listener, no database and no graceful-shutdown ceremony beyond
// answering SIGTERM.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// csp is the whole policy, built once, and the same one a deployed game
// serves. 'wasm-unsafe-eval' is the single addition to the baseline policy:
// without it browsers refuse to compile WebAssembly, which is the lab. It
// allows WASM compilation only — JavaScript eval stays blocked.
const csp = "default-src 'self'; " +
	"script-src 'self' 'wasm-unsafe-eval'; " +
	"img-src 'self' data:; " +
	"frame-ancestors 'none'; " +
	"base-uri 'none'; " +
	"form-action 'self'"

func main() {
	addr := flag.String("addr", envOr("ADDR", "127.0.0.1:8080"), "address to listen on (env ADDR)")
	dir := flag.String("dir", envOr("WEB_DIR", "web"), "the web tree to serve (env WEB_DIR)")
	flag.Parse()

	if err := run(*addr, *dir); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func run(addr, dir string) error {
	// os.Root, not a plain path: the tree is a real boundary, so a symlink
	// under web/ cannot serve somebody's private key.
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("opening %s: %w", dir, err)
	}
	defer root.Close()
	tree := root.FS()

	if err := registerTypes(); err != nil {
		return err
	}

	version, err := treeVersion(tree)
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}
	reportMissing(tree)

	srv := &http.Server{
		Addr:              addr,
		Handler:           routes(tree, version),
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			log.Printf("serve: shutdown: %v", err)
		}
	}()

	log.Printf("serve: http://%s/ version %s", addr, version)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// routes is the whole URL surface: the page, and the files under it.
func routes(tree fs.FS, version string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		page, err := fs.ReadFile(tree, "templates/index.html")
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		// One substitution, not a template engine: nothing on this page comes
		// from a request, so html/template would only escape what is never
		// user input.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, strings.ReplaceAll(string(page), "{{version}}", version))
	})

	static, err := fs.Sub(tree, "static")
	if err != nil {
		log.Fatalf("serve: web/static: %v", err)
	}
	// The exact directory URL would list the files, so it answers 404.
	mux.HandleFunc("GET /static/{$}", http.NotFound)
	mux.Handle("GET /static/", http.StripPrefix("/static/", cacheImmutable(http.FileServerFS(static))))

	return logRequests(secureHeaders(mux))
}

// registerTypes teaches the file server about WebAssembly. .wasm is not in
// Go's built-in table on every platform, and a module served as
// application/octet-stream fails to instantiate with a message nobody can
// read — which is the whole failure, silently, in the browser console.
func registerTypes() error {
	if err := mime.AddExtensionType(".wasm", "application/wasm"); err != nil {
		return fmt.Errorf("registering .wasm: %w", err)
	}
	return nil
}

// cacheImmutable marks a response as never changing. Every asset URL carries
// the tree's version, so a rebuild changes the URL rather than the file behind
// one a browser has already promised itself never to ask for again.
func cacheImmutable(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}

// logRequests writes one line per request. It is how you find the 404 that
// left the canvas saying "Loading...".
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Microsecond))
	})
}

// secureHeaders sets the policy before the handler runs: a header set after
// WriteHeader is silently dropped.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// statusWriter remembers the status code for the request log.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// envOr returns the environment variable or the fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// reportMissing names what `make wasm` has not produced yet. It is what turns
// "Loading..." for ever into a line in the terminal saying which command to
// run.
func reportMissing(tree fs.FS) {
	for _, p := range []string{
		"static/js/wasm_exec.js",
		"static/lab.wasm",
	} {
		if _, err := fs.Stat(tree, p); err != nil {
			log.Printf("serve: %s is missing — run make wasm", p)
		}
	}
}

// treeVersion names the served tree, not this binary.
//
// The assets are on disk rather than embedded, so the binary's identity says
// nothing about them: rebuilding lab.wasm has to change the string, and
// rebuilding the server has to not. fs.WalkDir walks in lexical order, so the
// digest is stable across restarts — a map-ordered walk would hand out a new
// version on every boot and quietly defeat the year-long cache.
func treeVersion(tree fs.FS) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(tree, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		f, err := tree.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		io.WriteString(h, p)
		_, err = io.Copy(h, f)
		return err
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}
