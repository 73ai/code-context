package main

import (
	"os"
)

func main() {
	// Use the cobra command system instead of direct argument parsing
	if err := Execute(); err != nil {
		os.Exit(1)
	}
}