package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
)

// Config is every knob this binary has. After parseConfig returns, nothing
// else reads os.Getenv — the struct is the whole contract.
type Config struct {
	Addr     string
	Bouncers int // the most bouncers the world holds at once
	Crowd    int // the bouncers the world starts with
	Dir      string
	LogLevel slog.Level
	Players  int // the most players at once
	Tick     int // world ticks per second
}

// errUsage means the flag set already printed what was wrong, so main only
// has to pick the exit code.
var errUsage = errors.New("usage error")

// parseConfig reads the flags, each defaulting to its environment variable,
// and validates them before anything opens a socket. A flag beats its
// variable beats the built-in default, and every default is one that works.
func parseConfig(args []string, stderr io.Writer) (Config, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var c Config
	fs.StringVar(&c.Addr, "addr", cmp.Or(os.Getenv("ADDR"), "127.0.0.1:8080"), "address to listen on (env ADDR)")
	fs.StringVar(&c.Dir, "dir", cmp.Or(os.Getenv("WEB_DIR"), "web"), "the web tree to serve (env WEB_DIR)")
	bouncers := fs.String("bouncers", cmp.Or(os.Getenv("BOUNCERS"), "1000"), "most bouncers at once, 0 to 10000 (env BOUNCERS)")
	crowd := fs.String("crowd", cmp.Or(os.Getenv("CROWD"), "100"), "bouncers the world starts with (env CROWD)")
	level := fs.String("log-level", cmp.Or(os.Getenv("LOG_LEVEL"), "info"), "debug|info|warn|error (env LOG_LEVEL)")
	players := fs.String("players", cmp.Or(os.Getenv("PLAYERS"), "8"), "most players at once, 1 to 256 (env PLAYERS)")
	tick := fs.String("tick", cmp.Or(os.Getenv("TICK"), "30"), "world ticks per second, 1 to 120 (env TICK)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return Config{}, err
		}
		return Config{}, errUsage
	}
	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("argument %q: serve takes flags only", fs.Arg(0))
	}

	if err := c.LogLevel.UnmarshalText([]byte(*level)); err != nil {
		return Config{}, fmt.Errorf("log-level %q: want debug, info, warn, or error", *level)
	}
	var err error
	if c.Tick, err = number("tick", *tick, 1, 120); err != nil {
		return Config{}, err
	}
	if c.Players, err = number("players", *players, 1, 256); err != nil {
		return Config{}, err
	}
	if c.Bouncers, err = number("bouncers", *bouncers, 0, 10000); err != nil {
		return Config{}, err
	}
	if c.Crowd, err = number("crowd", *crowd, 0, 10000); err != nil {
		return Config{}, err
	}
	// Two settings that are really one: the crowd has to fit under the cap,
	// or the world would start over a limit it then enforces on everybody.
	if c.Crowd > c.Bouncers {
		return Config{}, fmt.Errorf("crowd %d: more than -bouncers %d allows — raise -bouncers or lower -crowd", c.Crowd, c.Bouncers)
	}
	return c, nil
}

// number parses a whole number inside lo to hi, naming the flag when it is
// not one.
func number(name, value string, lo, hi int) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < lo || n > hi {
		return 0, fmt.Errorf("%s %q: want a whole number from %d to %d", name, value, lo, hi)
	}
	return n, nil
}

// LogValue is what slog logs for a Config. There are no secrets in it, so it
// is all of it; the method is here so the boot line prints one group rather
// than a struct's fields.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("addr", c.Addr),
		slog.Int("bouncers", c.Bouncers),
		slog.Int("crowd", c.Crowd),
		slog.String("dir", c.Dir),
		slog.String("log_level", c.LogLevel.String()),
		slog.Int("players", c.Players),
		slog.Int("tick", c.Tick),
	)
}
