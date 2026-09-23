// Package hint is the vocabulary hint (spec §7): when a Filter selects
// nothing, name the tags and keys it mentions that no idea carries, so a typo
// is visible. Shared by `idea ls` and the Review TUI.
package hint

import (
	"context"

	"github.com/NurramoX/ideation/internal/client"
)

// Vocabulary parses src locally (for the hint only; the server stays the
// authority) and checks its tags and keys against /tags and /attributes. It
// returns one line per unknown name, e.g. "no ideas have the key 'tga'", or
// nil when every name exists or src does not parse.
func Vocabulary(ctx context.Context, c client.Client, src string) ([]string, error) {
	return nil, nil
}
