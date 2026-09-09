package main

import (
	"errors"
	"flag"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// The environment half uses t.Setenv, so none of these run in parallel.

func TestParseConfigStartsWithNothingSet(t *testing.T) {
	for _, name := range []string{"ADDR", "WEB_DIR", "TICK", "PLAYERS", "BOUNCERS", "CROWD", "LOG_LEVEL"} {
		t.Setenv(name, "")
	}
	c, err := parseConfig(nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Addr: "127.0.0.1:8080", Bouncers: 1000, Crowd: 100, Dir: "web", LogLevel: slog.LevelInfo, Players: 8, Tick: 30}
	if c != want {
		t.Errorf("parseConfig() = %+v, want %+v", c, want)
	}
}

func TestParseConfigPrecedence(t *testing.T) {
	t.Setenv("TICK", "20")
	t.Setenv("LOG_LEVEL", "debug")

	c, err := parseConfig(nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.Tick != 20 || c.LogLevel != slog.LevelDebug {
		t.Errorf("the environment did not set tick %d and level %v", c.Tick, c.LogLevel)
	}

	c, err = parseConfig([]string{"-tick", "60"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.Tick != 60 {
		t.Errorf("tick = %d with -tick 60 and TICK=20, want the flag to win", c.Tick)
	}
}

func TestParseConfigRefuses(t *testing.T) {
	t.Setenv("BOUNCERS", "")
	t.Setenv("CROWD", "")
	tests := []struct {
		name string
		args []string
		want string // a piece of the message, which has to name the flag
	}{
		{"a tick of zero", []string{"-tick", "0"}, "tick"},
		{"a tick past 120", []string{"-tick", "121"}, "tick"},
		{"a tick that is not a number", []string{"-tick", "fast"}, `tick "fast"`},
		{"no players", []string{"-players", "0"}, "players"},
		{"too many bouncers", []string{"-bouncers", "10001"}, "bouncers"},
		{"a crowd over the cap", []string{"-bouncers", "10", "-crowd", "11"}, "crowd 11"},
		{"a level that is not one", []string{"-log-level", "loud"}, "log-level"},
		{"an argument", []string{"web"}, `argument "web"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseConfig(tt.args, io.Discard)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %q, want it to mention %q", err, tt.want)
			}
		})
	}

	t.Run("an unknown flag is a usage error the flag set already printed", func(t *testing.T) {
		var out strings.Builder
		if _, err := parseConfig([]string{"-nope"}, &out); !errors.Is(err, errUsage) {
			t.Errorf("err = %v, want errUsage", err)
		}
		if !strings.Contains(out.String(), "-tick") {
			t.Error("the usage text does not list the flags")
		}
	})
	t.Run("-h is help, not an error", func(t *testing.T) {
		if _, err := parseConfig([]string{"-h"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
			t.Errorf("err = %v, want flag.ErrHelp", err)
		}
	})
}
