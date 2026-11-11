package tui

import (
	"context"
	"fmt"
	"lazypass/internal/config"
	"lazypass/internal/generator"
	"lazypass/internal/state"
	"lazypass/internal/task"
	"lazypass/internal/theme"
	"lazypass/internal/tui/shared"
)

func (s *screen) toggleCompact() {
	s.cfg.Compact = !s.cfg.Compact
	if s.cfgPath == "" {
		return
	}
	compact, path := s.cfg.Compact, s.cfgPath
	generation := s.compactSaveGeneration.Add(1)
	work := func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := config.Update(path, func(current *config.Config) {
			if s.compactSaveGeneration.Load() == generation {
				current.Compact = compact
			}
		})
		if err != nil && s.dispatch != nil {
			s.dispatch(ctx, func() { s.notify(shared.Warning, "Could not save compact mode") })
			return nil
		}
		return err
	}
	if s.tasks != nil {
		if _, err := s.tasks.Submit(task.Task{ID: fmt.Sprintf("save compact mode %d", generation), Run: work}); err != nil {
			s.notify(shared.Warning, "Could not save compact mode")
		}
	} else if err := work(context.Background()); err != nil {
		s.notify(shared.Warning, "Could not save compact mode")
	}
}

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
	compact := s.cfg.Compact
	current, err := config.Update(s.cfgPath, func(current *config.Config) {
		change(current)
		current.Compact = compact
	})
	if err != nil {
		return err
	}
	current.LastVault = s.cfg.LastVault
	s.cfg = current
	return nil
}
