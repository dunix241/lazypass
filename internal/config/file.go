package config

import (
	"bytes"
	"errors"
	"os"
	"sync"

	"lazypass/internal/util"

	"gopkg.in/yaml.v3"
)

var writeMu sync.Mutex

func encodeYAML(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// replaceUnchanged narrows external-edit races; callers hold writeMu to serialize
// in-process saves, migrations, and schema maintenance.
func replaceUnchanged(path string, original, updated []byte) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("config is not a regular file")
	}
	latest, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(original, latest) {
		return errors.New("config changed during maintenance")
	}
	return util.WritePrivateFile(path, updated)
}

// Update serializes a read-modify-write against other in-process config writers.
func Update(path string, change func(*Config)) (Config, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	current, err := load(path)
	if err != nil {
		return Config{}, err
	}
	change(&current)
	if err := current.save(path); err != nil {
		return Config{}, err
	}
	return current, nil
}
