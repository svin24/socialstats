package main

import (
	"os"

	"socialstats/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args))
}
