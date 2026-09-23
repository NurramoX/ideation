// Package daemon runs the server in the foreground (spec §6): the home
// directory, the single-instance lock, the Unix socket and orderly shutdown.
// It knows nothing about launchd.
package daemon

import (
	"context"
	"errors"
	"io"

	"github.com/NurramoX/ideation/internal/home"
)

// ErrAlreadyRunning: another process holds daemon.lock.
var ErrAlreadyRunning = errors.New("already running")

// Run serves until ctx is cancelled (the caller cancels it on SIGTERM or
// SIGINT), then shuts down in the spec's order and returns nil. Lifecycle
// events and errors go to log.
func Run(ctx context.Context, h home.Home, log io.Writer) error {
	return errors.New("daemon.Run: not implemented")
}
