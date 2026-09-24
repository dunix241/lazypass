// Package cmd owns Cobra parsing, stream serialization, and exit boundaries.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"lazypass/internal/app"
	"lazypass/internal/build"
	"lazypass/internal/clipboard"
	"lazypass/internal/config"
	"lazypass/internal/tui"
	"lazypass/internal/util"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var rootCmd = &cobra.Command{
	Use:   "lazypass",
	Short: "A simple password generator with TUI and CLI modes",
	RunE:  runDefault,
}

func Execute() error { return rootCmd.Execute() }

// ExitCode returns 2 for invalid usage, 1 for runtime failures.
func ExitCode(err error) int {
	var usage *usageError
	if errors.As(err, &usage) {
		return 2
	}
	return 1
}

type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }
func usage(err error) error         { return &usageError{err: err} }

func init() {
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usage(err) })
	addOptions(rootCmd)
	rootCmd.PersistentFlags().BoolP("version", "v", false, "print lazypass version")
	rootCmd.PersistentFlags().String("config", "", "config file (default: XDG path)")
	rootCmd.PersistentFlags().Bool("copy", false, "copy the first generated password")
	rootCmd.PersistentFlags().Bool("no-tui", false, "generate instead of starting the TUI")
	rootCmd.PersistentFlags().Bool("stdout", false, "legacy alias for --no-tui")

	generate := &cobra.Command{Use: "generate", Short: "Generate passwords for a shell or script", RunE: runGenerate}
	generate.Flags().Int("count", 1, "number of passwords to generate")
	generate.Flags().String("format", "text", "output format: text or json")
	generate.Flags().Bool("no-save", false, "do not persist explicitly supplied options")

	tuiCommand := &cobra.Command{Use: "tui", Short: "Open the interactive password generator", RunE: runTUI}
	configCommand := &cobra.Command{Use: "config", Short: "Inspect lazypass configuration"}
	configCommand.AddCommand(
		&cobra.Command{Use: "path", Short: "Print the resolved configuration path", RunE: runConfigPath},
		&cobra.Command{Use: "show", Short: "Print saved password-generation options", RunE: runConfigShow},
	)
	version := &cobra.Command{Use: "version", Short: "Print lazypass version", Run: func(cmd *cobra.Command, _ []string) { fmt.Fprintln(cmd.OutOrStdout(), build.Version) }}
	rootCmd.AddCommand(generate, tuiCommand, configCommand, version)
}

func addOptions(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.IntP("length", "l", 0, "password length 4..256")
	flags.Bool("upper", false, "include uppercase letters")
	flags.Bool("no-upper", false, "exclude uppercase letters")
	flags.Bool("lower", false, "include lowercase letters")
	flags.Bool("no-lower", false, "exclude lowercase letters")
	flags.Bool("numbers", false, "include numbers")
	flags.Bool("no-numbers", false, "exclude numbers")
	flags.Bool("symbols", false, "include symbols")
	flags.Bool("no-symbols", false, "exclude symbols")
	flags.Bool("exclude-ambiguous", false, "exclude I, l, 1, O, and 0")
	flags.String("symbol-set", "", "symbols to use with --symbols")
}

func runDefault(cmd *cobra.Command, _ []string) error {
	if mustBool(cmd, "version") {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), build.Version)
		return err
	}
	if isTTY() && !mustBool(cmd, "no-tui") && !mustBool(cmd, "stdout") {
		return runTUI(cmd, nil)
	}
	return runGenerate(cmd, nil)
}

func runTUI(cmd *cobra.Command, _ []string) error {
	cfg, path, err := loadResolved(cmd)
	if err != nil {
		return err
	}
	if err := validateOverrides(cmd); err != nil {
		return usage(err)
	}
	opts, err := app.Resolve(cfg, overrides(cmd))
	if err != nil {
		return usage(err)
	}
	ui := tui.NewApp(config.FromOptions(opts), path, clipboard.Copy)
	if err := ui.Init(); err != nil {
		return err
	}
	return ui.Run()
}

func runGenerate(cmd *cobra.Command, _ []string) error {
	cfg, path, err := loadResolved(cmd)
	if err != nil {
		return err
	}
	if err := validateOverrides(cmd); err != nil {
		return usage(err)
	}
	opts, err := app.Resolve(cfg, overrides(cmd))
	if err != nil {
		return usage(err)
	}
	count := 1
	if cmd.Flags().Lookup("count") != nil {
		count = mustInt(cmd, "count")
	}
	results, err := app.Generate(opts, count)
	if err != nil {
		return usage(err)
	}
	format := "text"
	if cmd.Flags().Lookup("format") != nil {
		format = mustString(cmd, "format")
	}
	if err := writeResults(cmd, results, format); err != nil {
		return usage(err)
	}

	noSave := cmd.Flags().Lookup("no-save") != nil && mustBool(cmd, "no-save")
	if hasOptionOverrides(cmd) && !noSave {
		if err := config.FromOptions(opts).Save(path); err != nil {
			return fmt.Errorf("saving options: %w", err)
		}
	}
	if mustBool(cmd, "copy") {
		if err := clipboard.Copy(results[0].Password); err != nil {
			return fmt.Errorf("copying generated password: %w", err)
		}
	}
	return nil
}

func writeResults(cmd *cobra.Command, results []app.Result, format string) error {
	switch format {
	case "text":
		for _, result := range results {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), result.Password); err != nil {
				return err
			}
		}
		return nil
	case "json":
		encoder := json.NewEncoder(cmd.OutOrStdout())
		for _, result := range results {
			if err := encoder.Encode(result); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("invalid format %q: use text or json", format)
	}
}

func runConfigPath(cmd *cobra.Command, _ []string) error {
	path, err := config.ResolvePath(mustString(cmd, "config"))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
	return err
}

func runConfigShow(cmd *cobra.Command, _ []string) error {
	cfg, _, err := loadResolved(cmd)
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write(data)
	return err
}

func loadResolved(cmd *cobra.Command) (config.Config, string, error) {
	path, err := config.ResolvePath(mustString(cmd, "config"))
	if err != nil {
		return config.Config{}, "", err
	}
	cfg, err := config.Load(path)
	return cfg, path, err
}

func overrides(cmd *cobra.Command) app.Overrides {
	var result app.Overrides
	if changed(cmd, "length") {
		value := mustInt(cmd, "length")
		result.Length = &value
	}
	result.Upper = boolOverride(cmd, "upper", "no-upper")
	result.Lower = boolOverride(cmd, "lower", "no-lower")
	result.Numbers = boolOverride(cmd, "numbers", "no-numbers")
	result.Symbols = boolOverride(cmd, "symbols", "no-symbols")
	if changed(cmd, "exclude-ambiguous") {
		value := mustBool(cmd, "exclude-ambiguous")
		result.ExcludeAmbiguous = &value
	}
	if changed(cmd, "symbol-set") {
		value := mustString(cmd, "symbol-set")
		result.SymbolSet = &value
	}
	return result
}

func boolOverride(cmd *cobra.Command, include, exclude string) *bool {
	if changed(cmd, include) {
		value := mustBool(cmd, include)
		return &value
	}
	if changed(cmd, exclude) {
		value := !mustBool(cmd, exclude)
		return &value
	}
	return nil
}

func validateOverrides(cmd *cobra.Command) error {
	for _, pair := range [][2]string{{"upper", "no-upper"}, {"lower", "no-lower"}, {"numbers", "no-numbers"}, {"symbols", "no-symbols"}} {
		if changed(cmd, pair[0]) && changed(cmd, pair[1]) {
			return fmt.Errorf("cannot use --%s and --%s together", pair[0], pair[1])
		}
	}
	return nil
}

func hasOptionOverrides(cmd *cobra.Command) bool {
	for _, name := range []string{"length", "upper", "no-upper", "lower", "no-lower", "numbers", "no-numbers", "symbols", "no-symbols", "exclude-ambiguous", "symbol-set"} {
		if changed(cmd, name) {
			return true
		}
	}
	return false
}

func changed(cmd *cobra.Command, name string) bool {
	return cmd.Flags().Changed(name) || cmd.InheritedFlags().Changed(name)
}
func mustBool(cmd *cobra.Command, name string) bool {
	value, _ := cmd.Flags().GetBool(name)
	return value
}
func mustInt(cmd *cobra.Command, name string) int { value, _ := cmd.Flags().GetInt(name); return value }
func mustString(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return value
}
func isTTY() bool { return util.IsTerminal(os.Stdout) }
