// Command serve hosts the lab: one page and the web tree, under the same
// security headers and the same cache contract a deployed game gets, and the
// world the lab plays in, over GET /ws.
//
// The page and the tree are read from disk, so editing the stylesheet and
// reloading is the whole loop. The world runs on a fixed tick in one
// goroutine, because the engine is not safe for concurrent use and was never
// meant to be; every socket feeds it intents and gets the world back. It
// answers SIGTERM by closing every socket with 1001 and waiting for them.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := parseConfig(os.Args[1:], os.Stderr)
	switch {
	case errors.Is(err, flag.ErrHelp):
		return
	case errors.Is(err, errUsage):
		os.Exit(2)
	case err != nil:
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(2)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	if err := run(cfg, logger); err != nil {
		logger.Error("serve", "error", err)
		os.Exit(1)
	}
}

func run(cfg Config, logger *slog.Logger) error {
	// os.Root, not a plain path: the tree is a real boundary, so a symlink
	// under web/ cannot serve somebody's private key.
	root, err := os.OpenRoot(cfg.Dir)
	if err != nil {
		return fmt.Errorf("opening %s: %w", cfg.Dir, err)
	}
	defer root.Close()
	tree := root.FS()

	if err := registerTypes(); err != nil {
		return err
	}
	version, err := treeVersion(tree)
	if err != nil {
		return fmt.Errorf("reading %s: %w", cfg.Dir, err)
	}
	reportMissing(tree, logger)

	w := newWorld(cfg, logger)
	handler, err := routes(tree, version, logger, w)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.Addr,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		Handler:           handler,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	// Shutdown never drains a hijacked connection, so the sockets are told
	// to go here and waited for below.
	srv.RegisterOnShutdown(w.hub.closeAll)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Two goroutines under one context, each with a wait of its own: what
	// errgroup would do, in a module whose feature is having no dependencies.
	worldDone := make(chan struct{})
	go func() {
		defer close(worldDone)
		w.run(ctx)
	}()
	served := make(chan error, 1)
	go func() { served <- srv.ListenAndServe() }()

	logger.Info("serving", "url", "http://"+cfg.Addr+"/", "version", version, "config", cfg)
	select {
	case <-ctx.Done():
	case err = <-served:
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		stop()
	}

	// A fresh deadline: the signal context is already over.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if serr := srv.Shutdown(shutdownCtx); serr != nil && err == nil {
		err = serr
	}
	w.hub.Wait()
	<-worldDone
	return err
}
