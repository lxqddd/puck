package history

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStoreRestrictsPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "puck")
	path := filepath.Join(dir, "history.db")

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	for _, name := range []string{dir, path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s has permissions %04o; expected no group/other access", name, info.Mode().Perm())
		}
	}
}
