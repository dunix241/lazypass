// Command lazypass generates passwords and manages encrypted vaults.
package main

import (
	"fmt"
	"os"
	"runtime"

	"lazypass/cmd"
	"lazypass/internal/process"
)

func main() {
	os.Args = process.ApplicationArgs(runtime.GOOS, os.Args)
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
