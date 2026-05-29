package main

import (
	"os"
	"github.com/joshuademarco/glyph/internal/cli"
)

// version is overridden at build time via:
//
//	go build -ldflags "-X main.version=$(git describe --tags)"
var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
