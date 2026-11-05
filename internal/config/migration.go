package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Legacy syntax is transformed before decoding the canonical configuration.
func migrateVaultConfig(data []byte) (*yaml.Node, bool, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, false, err
	}
	if len(document.Content) == 0 {
		return &document, false, nil
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return &document, false, nil
	}
	legacy, canonical := -1, -1
	for i := 0; i+1 < len(root.Content); i += 2 {
		switch root.Content[i].Value {
		case "vault":
			if legacy >= 0 {
				return nil, false, fmt.Errorf("duplicate vault key")
			}
			legacy = i
		case "vaults":
			canonical = i
		}
	}
	changed := legacy >= 0
	if legacy >= 0 {
		if canonical >= 0 {
			root.Content = append(root.Content[:legacy], root.Content[legacy+2:]...)
		} else {
			root.Content[legacy].Value = "vaults"
		}
	}
	// Also normalize the old mapping form under an already-renamed vaults key.
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "vaults" {
			continue
		}
		value := root.Content[i+1]
		if value.Kind == yaml.MappingNode {
			root.Content[i+1] = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{value}}
			changed = true
		}
	}
	return &document, changed, nil
}
