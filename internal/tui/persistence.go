package tui

import (
	"lazypass/internal/config"
	"lazypass/internal/generator"
	"lazypass/internal/state"
	"lazypass/internal/theme"
)

func (s *screen) save() {
	_ = s.saveVaultSelection()
	_ = s.saveOptions(s.generator.Options())
}

func (s *screen) saveVaultSelection() error {
	name := s.vaults[s.vaultIndex].Name
	s.cfg.LastVault = name
	return state.SaveVault(s.cfgPath, name)
}

func (s *screen) saveOptions(options generator.Options) error {
	if s.cfgPath == "" {
		return nil
	}
	return s.updateConfig(func(current *config.Config) { *current = current.WithOptions(options) })
}

// The deleted theme must stop being the live selection even if persistence fails.
func (s *screen) useDefaultTheme() error {
	s.cfg.Theme = theme.DefaultName()
	return s.saveTheme(s.cfg.Theme)
}

func (s *screen) saveTheme(name string) error {
	if s.cfgPath == "" {
		s.cfg.Theme = name
		return nil
	}
	return s.updateConfig(func(current *config.Config) { current.Theme = name })
}

func (s *screen) updateConfig(change func(*config.Config)) error {
	current, err := config.Update(s.cfgPath, change)
	if err != nil {
		return err
	}
	current.LastVault = s.cfg.LastVault
	s.cfg = current
	return nil
}
