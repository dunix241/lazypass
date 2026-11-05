package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type UpdateMode string

const (
	UpdatesOff    UpdateMode = "off"
	UpdatesNotify UpdateMode = "notify"
	UpdatesAuto   UpdateMode = "auto"
)

func (m *UpdateMode) UnmarshalYAML(node *yaml.Node) error {
	// Preserve the previous opt-out when reading an existing configuration.
	if node.Kind == yaml.MappingNode {
		legacy := struct {
			Check bool `yaml:"check"`
		}{Check: true}
		if err := node.Decode(&legacy); err != nil {
			return err
		}
		*m = UpdatesNotify
		if !legacy.Check {
			*m = UpdatesOff
		}
		return nil
	}
	switch UpdateMode(node.Value) {
	case UpdatesOff, UpdatesNotify, UpdatesAuto:
		*m = UpdateMode(node.Value)
		return nil
	default:
		return fmt.Errorf("updates must be off, notify, or auto")
	}
}
