package main

import (
	"log"

	"github.com/luxfi/genesis/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}