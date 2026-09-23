// Package cli is the `idea` command line (spec §7): hand-rolled verb
// dispatch, output, exit codes, launchd management and completion.
package cli

import (
	"io"

	_ "golang.org/x/term"
)

// Main runs `idea` with args (without the program name) and returns the exit
// code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return 1
}
