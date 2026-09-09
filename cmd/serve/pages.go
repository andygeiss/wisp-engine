package main

// The static half of the server: the page, the tree under it, the headers
// every response carries, and the version that busts the cache.

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"
)

// csp is the whole policy, built once, and the same one a deployed game
// serves. 'wasm-unsafe-eval' is the single addition to the baseline policy:
// without it browsers refuse to compile WebAssembly, which is the lab. It
// allows WASM compilation only — JavaScript eval stays blocked. A ws:
// connection to the page's own host falls under default-src 'self'.
const csp = "default-src 'self'; " +
	"script-src 'self' 'wasm-unsafe-eval'; " +
	"img-src 'self' data:; " +
	"frame-ancestors 'none'; " +
	"base-uri 'none'; " +
	"form-action 'self'"

// routes is the whole URL surface: the page, the files under it, and the
// socket the world is played over.
func routes(tree fs.FS, version string, logger *slog.Logger, w *world) (http.Handler, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(rw http.ResponseWriter, r *http.Request) {
		page, err := fs.ReadFile(tree, "templates/index.html")
		if err != nil {
			http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		// One substitution, not a template engine: nothing on this page comes
		// from a request, so html/template would only escape what is never
		// user input.
		rw.Header().Set("Cache-Control", "no-store")
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(rw, strings.ReplaceAll(string(page), "{{version}}", version))
	})

	static, err := fs.Sub(tree, "static")
	if err != nil {
		return nil, fmt.Errorf("web/static: %w", err)
	}
	// The exact directory URL would list the files, so it answers 404.
	mux.HandleFunc("GET /static/{$}", http.NotFound)
	mux.Handle("GET /static/", http.StripPrefix("/static/", cacheImmutable(http.FileServerFS(static))))

	mux.HandleFunc("GET /ws", w.serveWS)

	// The body cap and the cross-origin check have nothing to do on a server
	// that only answers GET. They are here so that the day it answers
	// anything else, they already are.
	h := http.MaxBytesHandler(mux, 1<<20)
	h = http.NewCrossOriginProtection().Handler(h)
	return logRequests(logger, secureHeaders(h)), nil
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
// left the canvas saying "Loading...". A socket's line comes when the socket
// closes, with 101 and how long it lived.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.status,
			"duration", time.Since(start).Round(time.Microsecond))
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

// WriteHeader records the status and passes it on.
func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Unwrap lets http.ResponseController reach the writer underneath for
// everything this wrapper does not do itself.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Hijack hands the connection over for a WebSocket and records that it did,
// so the request log says 101 rather than a 200 nobody sent.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	nc, brw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.status = http.StatusSwitchingProtocols
	}
	return nc, brw, err
}

// reportMissing names what `make wasm` has not produced yet. It is what turns
// "Loading..." for ever into a line in the terminal saying which command to
// run.
func reportMissing(tree fs.FS, logger *slog.Logger) {
	for _, p := range []string{
		"static/js/wasm_exec.js",
		"static/lab.wasm",
	} {
		if _, err := fs.Stat(tree, p); err != nil {
			logger.Warn("browser artefact missing — run make wasm", "path", p)
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
