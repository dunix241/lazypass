package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"lazypass/internal/build"
	"lazypass/internal/state"
	"lazypass/internal/update"
)

func runUpdateCheck(cmd *cobra.Command, _ []string) error {
	if !update.Stable(build.Version) {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Installed: %s (development build; stable release comparison skipped)\n", build.Version)
		return err
	}
	checker := update.Checker{Source: update.GitHub{}, Store: state.UpdateStore{}}
	release, available, err := checker.Check(cmd.Context(), build.Version, true)
	if err != nil {
		return err
	}
	if available {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Installed: %s\nAvailable: %s\n%s\nUpdate using your package manager or the release download.\n", build.Version, release.Version, release.URL)
	} else {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Installed: %s\nLatest: %s\nYou are up to date.\n", build.Version, release.Version)
	}
	return err
}
