package main

import (
	"os"

	"github.com/NurramoX/ideation/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
