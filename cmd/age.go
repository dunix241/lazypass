package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"lazypass/internal/config"
	ageprovider "lazypass/internal/vault/age"
	"lazypass/internal/vault/providers"
)

func terminalPassphrase(label string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("an interactive terminal is required to unlock the age identity")
	}
	defer tty.Close()
	if !term.IsTerminal(int(tty.Fd())) {
		return "", fmt.Errorf("an interactive terminal is required")
	}
	fmt.Fprint(tty, label)
	value, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(tty)
	if err != nil {
		return "", fmt.Errorf("reading identity passphrase failed")
	}
	defer clear(value)
	return string(value), nil
}

func runVaultInit(cmd *cobra.Command, _ []string) error {
	cfg, path, err := loadResolved(cmd)
	if err != nil {
		return err
	}
	provider := mustString(cmd, "provider")
	name := mustString(cmd, "name")
	if name == "" {
		name = provider
	}
	if err := config.ValidateVaultName(name); err != nil {
		return usage(fmt.Errorf("vault names must use lowercase letters, numbers, and hyphens"))
	}
	if _, exists := cfg.Vaults.Select(name); exists {
		return fmt.Errorf("vault name already exists")
	}
	if provider == "pass" {
		return initPassVault(cmd, cfg, path)
	}
	if provider == "bitwarden" {
		return initBitwardenVault(cmd, cfg, path, name)
	}
	if provider != "age" {
		return usage(fmt.Errorf("unsupported vault provider: use age, pass, or bitwarden"))
	}
	entry := config.Vault{Name: name, Provider: "age"}
	if value := mustString(cmd, "store-dir"); value != "" {
		entry.StoreDir = value
	}
	if value := mustString(cmd, "identity-file"); value != "" {
		entry.IdentityFile = value
	}
	store, identity, err := providers.AgePaths(entry)
	if err != nil {
		return err
	}
	if recipient := mustString(cmd, "recipient"); recipient != "" {
		if err := ageprovider.InitializeRecipient(store, recipient); err != nil {
			return err
		}
	} else {
		passphrase, err := terminalPassphrase("New age identity passphrase: ")
		if err != nil {
			return err
		}
		confirm, err := terminalPassphrase("Confirm passphrase: ")
		if err != nil {
			return err
		}
		if passphrase == "" || passphrase != confirm {
			return fmt.Errorf("passphrases must be nonempty and match")
		}
		if err := ageprovider.Initialize(store, identity, passphrase); err != nil {
			return err
		}
	}
	entry.StoreDir, entry.IdentityFile = store, identity
	cfg.Vaults = append(cfg.Vaults, entry)
	if err := cfg.Save(path); err != nil {
		return fmt.Errorf("vault initialized, but saving config failed: %w", err)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "Age vault initialized. Back up the encrypted identity and its passphrase separately.")
	return err
}
