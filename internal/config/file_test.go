package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentUpdatesAndSchemaMaintenance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Defaults().Save(path); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := Update(path, func(c *Config) { c.Length++ }); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := SyncSchema(context.Background(), path); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Length != Defaults().Length+20 {
		t.Fatalf("updates lost: length=%d", cfg.Length)
	}
}

func TestMaintenanceRejectsChangedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("length: 32\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := replaceUnchanged(path, []byte("length: 20\n"), []byte("length: 24\n")); err == nil {
		t.Fatal("overwrote external edit")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "length: 32\n" {
		t.Fatalf("external edit changed: %q", data)
	}
}
