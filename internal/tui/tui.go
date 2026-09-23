// Package tui is the Review TUI (spec §8).
package tui

import (
	"context"
	"errors"

	"github.com/NurramoX/ideation/internal/client"

	_ "charm.land/bubbles/v2/textinput"
	_ "charm.land/bubbles/v2/viewport"
	_ "charm.land/bubbletea/v2"
	_ "charm.land/glamour/v2"
	_ "charm.land/lipgloss/v2"
)

// DefaultFilter is the Review queue, prefilled when no Filter is given.
const DefaultFilter = "status:raw,active -reviewed:90d.."

// Run shows the Review TUI on the terminal, starting from filter ("" means
// DefaultFilter). It returns after the terminal is restored. aborted holds
// the user's text from every editor round-trip aborted after a 412, which the
// caller prints to stdout.
func Run(ctx context.Context, c client.Client, filter string) (aborted [][]byte, err error) {
	return nil, errors.New("tui.Run: not implemented")
}
