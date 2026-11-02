// Package state persists session choices separately from user configuration.
package state

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"lazypass/internal/util"

	"gopkg.in/yaml.v3"
)

var mu sync.Mutex

type session struct {
	Vaults map[string]string `yaml:"lastVaults"`
}

func path() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "lazypass", "state.yaml"), nil
}

func load(file string) (session, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return session{Vaults: map[string]string{}}, nil
	}
	if err != nil {
		return session{}, err
	}
	var result session
	if err := yaml.Unmarshal(data, &result); err != nil {
		return session{}, err
	}
	if result.Vaults == nil {
		result.Vaults = map[string]string{}
	}
	return result, nil
}

func LastVault(configPath string) (string, error) {
	if configPath == "" {
		return "", nil
	}
	key, err := filepath.Abs(configPath)
	if err != nil {
		return "", err
	}
	mu.Lock()
	defer mu.Unlock()
	file, err := path()
	if err != nil {
		return "", err
	}
	value, err := load(file)
	if err != nil {
		return "", err
	}
	return value.Vaults[key], nil
}

func SaveVault(configPath, name string) error {
	if configPath == "" || name == "" {
		return nil
	}
	key, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	file, err := path()
	if err != nil {
		return err
	}
	value, err := load(file)
	if err != nil {
		return err
	}
	if value.Vaults[key] == name {
		return nil
	}
	value.Vaults[key] = name
	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	return util.WritePrivateFile(file, data)
}

// ResolveVault migrates a legacy selection only when no state has been saved.
func ResolveVault(configPath, legacy string) (string, error) {
	selected, err := LastVault(configPath)
	if err != nil || selected != "" {
		return selected, err
	}
	return legacy, SaveVault(configPath, legacy)
}
