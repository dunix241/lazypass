package main

import (
	"fmt"
	"os"

	"lazypass/cmd"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\nERROR: lazypass crashed unexpectedly: %v\n", r)
			os.Exit(1)
		}
	}()

	if err := cmd.Execute(); err != nil {
		os.Exit(cmd.ExitCode(err))
	}
}
