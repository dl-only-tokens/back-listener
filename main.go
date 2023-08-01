package main

import (
	"os"

	"github.com/dl-only-tokens/back-listener/internal/cli"
)

func main() {
	if !cli.Run(os.Args) {
		os.Exit(1)
	}
}
