package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Legacy syntax is transformed before decoding the canonical configuration.
func migrateConfig(data []byte) (*yaml.Node, bool, error) {
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
		value := root.Content[i+1]
		switch root.Content[i].Value {
		case "updates":
			migrated, err := migrateUpdates(value)
			if err != nil {
				return nil, false, err
			}
			changed = changed || migrated
		case "vaults":
			if value.Kind == yaml.MappingNode {
				root.Content[i+1] = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{value}}
				changed = true
			}
		}
	}
	return &document, changed, nil
}

func migrateUpdates(node *yaml.Node) (bool, error) {
	settings := defaultUpdates()
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Tag != "!!str" {
			return false, fmt.Errorf("updates must be a mapping")
		}
		if err := node.Decode(&settings.Mode); err != nil {
			return false, err
		}
	case yaml.MappingNode:
		if len(node.Content) != 2 || node.Content[0].Value != "check" {
			return false, nil
		}
		var enabled bool
		if err := node.Content[1].Decode(&enabled); err != nil {
			return false, err
		}
		if node.Content[1].Tag != "!!bool" {
			return false, fmt.Errorf("updates.check must be a boolean")
		}
		if !enabled {
			settings.Mode = UpdatesOff
		}
	default:
		return false, nil
	}
	var replacement yaml.Node
	if err := replacement.Encode(settings); err != nil {
		return false, err
	}
	if node.Kind == yaml.MappingNode {
		replacement.Content[0].HeadComment = node.Content[0].HeadComment
		replacement.Content[1].LineComment = node.Content[1].LineComment
	}
	replacement.HeadComment, replacement.LineComment, replacement.FootComment = node.HeadComment, node.LineComment, node.FootComment
	*node = replacement
	return true, nil
}
