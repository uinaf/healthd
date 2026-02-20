package main

import (
	"os"

	"github.com/uinaf/healthd/cmd"
)

func main() {
	if err := cmd.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
