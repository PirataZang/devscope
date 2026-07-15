package main

import (
	"os"

	"github.com/devscope/devscope/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
