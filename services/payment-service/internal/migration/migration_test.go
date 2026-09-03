package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverReturnsUpMigrationsInVersionOrder(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"000002_second.up.sql":   "SELECT 2;",
		"000001_first.up.sql":    "SELECT 1;",
		"000001_first.down.sql":  "SELECT 0;",
		"not-a-migration.up.sql": "SELECT bad;",
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write test migration: %v", err)
		}
	}

	migrations, err := Discover(dir)
	if err == nil {
		t.Fatalf("Discover() error = nil, want invalid filename error")
	}

	if err := os.Remove(filepath.Join(dir, "not-a-migration.up.sql")); err != nil {
		t.Fatalf("remove invalid migration: %v", err)
	}

	migrations, err = Discover(dir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("len(migrations) = %d, want 2", len(migrations))
	}
	if migrations[0].Version != 1 || migrations[1].Version != 2 {
		t.Fatalf("migration order = %d, %d; want 1, 2", migrations[0].Version, migrations[1].Version)
	}
}
