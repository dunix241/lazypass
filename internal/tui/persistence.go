package tui

import (
	"lazypass/internal/config"
	"lazypass/internal/generator"
	"lazypass/internal/theme"
)

func (s *screen) save() { _ = s.saveOptions(s.generator.Options()) }

func (s *screen) saveOptions(options generator.Options) error {
	if s.cfgPath == "" {
		return nil
	}
	return s.updateConfig(func(current config.Config) config.Config { return current.WithOptions(options) })
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
	return s.updateConfig(func(current config.Config) config.Config { current.Theme = name; return current })
}

func (s *screen) updateConfig(update func(config.Config) config.Config) error {
	current, err := config.Load(s.cfgPath)
	if err != nil {
		return err
	}
	current = update(current)
	if err := current.Save(s.cfgPath); err != nil {
		return err
	}
	s.cfg = current
	return nil
}
