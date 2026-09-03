package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const lockKey int64 = 2026090301

type Migration struct {
	Version int64
	Name    string
	Path    string
}

func Run(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	migrationsDir, err := ResolveDir(dir)
	if err != nil {
		return err
	}

	migrations, err := Discover(migrationsDir)
	if err != nil {
		return err
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version BIGINT PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}

	applied, err := appliedVersions(ctx, tx)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if applied[migration.Version] {
			continue
		}

		sql, err := os.ReadFile(migration.Path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", migration.Name, err)
		}

		if strings.TrimSpace(string(sql)) != "" {
			if _, err := tx.Exec(ctx, string(sql)); err != nil {
				return fmt.Errorf("apply migration %s: %w", migration.Name, err)
			}
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", migration.Version, migration.Name); err != nil {
			return fmt.Errorf("record migration %s: %w", migration.Name, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}

	return nil
}

func ResolveDir(dir string) (string, error) {
	if strings.TrimSpace(dir) != "" {
		return existingDir(dir)
	}

	candidates := []string{
		"migrations",
		filepath.Join("..", "..", "migrations"),
		filepath.Join("..", "..", "..", "migrations"),
	}

	for _, candidate := range candidates {
		resolved, err := existingDir(candidate)
		if err == nil {
			return resolved, nil
		}
	}

	return "", fmt.Errorf("migrations directory not found")
}

func Discover(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		migration, err := parseMigration(filepath.Join(dir, entry.Name()), entry.Name())
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, migration)
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func appliedVersions(ctx context.Context, tx pgx.Tx) (map[int64]bool, error) {
	rows, err := tx.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	defer rows.Close()

	applied := map[int64]bool{}
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}

	return applied, nil
}

func parseMigration(path, name string) (Migration, error) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) != 2 {
		return Migration{}, fmt.Errorf("invalid migration filename %q", name)
	}

	version, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Migration{}, fmt.Errorf("invalid migration version %q: %w", name, err)
	}

	return Migration{
		Version: version,
		Name:    strings.TrimSuffix(name, ".up.sql"),
		Path:    path,
	}, nil
}

func existingDir(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve migrations directory %q: %w", dir, err)
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", absolute)
	}

	return absolute, nil
}
