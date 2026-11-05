package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"lazypass/internal/build"
	"lazypass/internal/install"
	"lazypass/internal/state"
	"lazypass/internal/update"
)

func runUpdate(cmd *cobra.Command, _ []string) error {
	owner, err := install.Detect(cmd.Context())
	if err != nil {
		return err
	}
	if owner.Guidance != "" {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), owner.Guidance)
		return err
	}
	adopt, _ := cmd.Flags().GetBool("adopt")
	if err := owner.ValidateUpdate(adopt); err != nil {
		return err
	}
	release, err := (update.GitHub{}).Latest(cmd.Context())
	if err != nil {
		return err
	}
	current := build.Version
	if owner.Method == "script" {
		current = owner.Release
	}
	if update.Newer(current, release.Version) {
		return fmt.Errorf("installed version %s is newer than latest stable %s; downgrades are not supported", current, release.Version)
	}
	if owner.Method == "script" && !update.Newer(release.Version, current) {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Already up to date: %s\n", current)
		return err
	}
	return install.Delegate(cmd.Context(), owner, release.Version, adopt, cmd.OutOrStdout(), cmd.ErrOrStderr())
}

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
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Installed: %s\nAvailable: %s\n%s\nRun lazypass update for installation-specific guidance.\n", build.Version, release.Version, release.URL)
	} else {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Installed: %s\nLatest: %s\nYou are up to date.\n", build.Version, release.Version)
	}
	return err
}
