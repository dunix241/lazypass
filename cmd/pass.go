package cmd

import (
	"fmt"

	"lazypass/internal/config"
	passprovider "lazypass/internal/vault/pass"

	"github.com/spf13/cobra"
)

func initPassVault(cmd *cobra.Command, cfg config.Config, path string) error {
	name := mustString(cmd, "name")
	if name == "" {
		name = "pass"
	}
	entry := config.Vault{Name: name, Provider: "pass"}
	if mustString(cmd, "identity-file") != "" {
		return usage(fmt.Errorf("--identity-file applies only to age"))
	}
	recipient := mustString(cmd, "recipient")
	if recipient == "" {
		return usage(fmt.Errorf("pass initialization requires --recipient <GPG fingerprint>"))
	}
	if store := mustString(cmd, "store-dir"); store != "" {
		entry.StoreDir = store
	}
	p := passprovider.New(entry.StoreDir, nil)
	if err := p.Initialize(cmd.Context(), recipient); err != nil {
		return err
	}
	entry.StoreDir = p.StoreDirectory()
	cfg.Vaults = append(cfg.Vaults, entry)
	if err := cfg.Save(path); err != nil {
		return fmt.Errorf("vault initialized, but saving config failed: %w", err)
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), "Pass vault initialized.")
	return err
}
