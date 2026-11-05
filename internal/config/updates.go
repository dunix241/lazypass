package config

import (
	"fmt"
	"time"

	"lazypass/internal/update"

	"gopkg.in/yaml.v3"
)

type Updates struct {
	Mode        UpdateMode    `yaml:"mode"`
	CacheMaxAge time.Duration `yaml:"cacheMaxAge"`
}

func defaultUpdates() Updates {
	return Updates{Mode: UpdatesNotify, CacheMaxAge: update.DefaultCacheMaxAge}
}

func (u *Updates) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("updates must be a mapping")
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if key != "mode" && key != "cacheMaxAge" {
			return fmt.Errorf("unknown updates setting %q", key)
		}
	}
	type plain Updates
	value := plain(defaultUpdates())
	if err := node.Decode(&value); err != nil {
		return err
	}
	if value.CacheMaxAge <= 0 {
		return fmt.Errorf("updates.cacheMaxAge must be a positive duration, such as 15m or 1h")
	}
	*u = Updates(value)
	return nil
}

type UpdateMode string

const (
	UpdatesOff    UpdateMode = "off"
	UpdatesNotify UpdateMode = "notify"
	UpdatesAuto   UpdateMode = "auto"
)

func (m *UpdateMode) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return fmt.Errorf("updates.mode must be off, notify, or auto")
	}
	switch UpdateMode(node.Value) {
	case UpdatesOff, UpdatesNotify, UpdatesAuto:
		*m = UpdateMode(node.Value)
		return nil
	default:
		return fmt.Errorf("updates.mode must be off, notify, or auto")
	}
}
