package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"lazypass/internal/config"
	"lazypass/internal/vault/bitwarden"
	"lazypass/internal/vault/providers"
	"net/url"
)

func initBitwardenVault(cmd *cobra.Command, cfg config.Config, path, name string) error {
	entry := config.Vault{Name: name, Provider: "bitwarden", Server: mustString(cmd, "server"), StoreDir: mustString(cmd, "store-dir")}
	if entry.Server == "" {
		entry.Server = bitwarden.DefaultServer
	}
	u, err := url.Parse(entry.Server)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return usage(fmt.Errorf("bitwarden server must be an HTTPS URL without credentials"))
	}
	p, err := providers.Builtins().Open(entry, nil)
	if err != nil {
		return err
	}
	if err := p.(*bitwarden.Provider).Initialize(cmd.Context()); err != nil {
		return err
	}
	entry.StoreDir = p.StoreDirectory()
	cfg.Vaults = append(cfg.Vaults, entry)
	if err := cfg.Save(path); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Vault configured. Log in using this isolated CLI directory:\nBITWARDENCLI_APPDATA_DIR=%q bw login\n", entry.StoreDir)
	return err
}
