// Package home locates the daemon's home directory and the files in it
// (spec §6). The daemon and the CLI read it the same way.
package home

import (
	"os"
	"path/filepath"
)

// EnvVar replaces the home root for tests and dev builds.
const EnvVar = "IDEATION_HOME"

// Label is the launchd job label.
const Label = "io.github.nurramox.ideation"

// Home is a resolved home directory.
type Home struct {
	Dir string
	// Custom is true when IDEATION_HOME set Dir.
	Custom bool
}

// Resolve returns $IDEATION_HOME when set, otherwise
// ~/Library/Application Support/ideation.
func Resolve() (Home, error) {
	if d := os.Getenv(EnvVar); d != "" {
		abs, err := filepath.Abs(d)
		if err != nil {
			return Home{}, err
		}
		return Home{Dir: abs, Custom: true}, nil
	}
	u, err := os.UserHomeDir()
	if err != nil {
		return Home{}, err
	}
	return Home{Dir: filepath.Join(u, "Library", "Application Support", "ideation")}, nil
}

func (h Home) DB() string     { return filepath.Join(h.Dir, "ideation.db") }
func (h Home) Socket() string { return filepath.Join(h.Dir, "ideation.sock") }
func (h Home) Lock() string   { return filepath.Join(h.Dir, "daemon.lock") }
