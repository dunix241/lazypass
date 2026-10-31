package util

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPrivateWritesAreAtomicAndLeaveNoTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	var wg sync.WaitGroup
	for _, value := range []string{"first complete document", "second complete document"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := WritePrivateFile(path, []byte(value)); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first complete document" && string(data) != "second complete document" {
		t.Fatal("partial document written")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("file not private")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary files left behind")
	}
}
