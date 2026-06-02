package main

import (
	"os"

	"github.com/gtkpad/devclean/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
