package main

import (
	"os"

	"github.com/luxfi/genesis/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}