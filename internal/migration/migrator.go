package migration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Migrator struct {
	db  *sql.DB
	dir string
}

func NewMigrator(db *sql.DB, dir string) *Migrator {
	return &Migrator{db: db, dir: dir}
}

func (m *Migrator) Up(ctx context.Context) error {
	if _, err := m.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	for _, file := range files {
		applied, err := m.isApplied(ctx, file)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		if err := m.apply(ctx, file); err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) isApplied(ctx context.Context, version string) (bool, error) {
	var exists bool
	err := m.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return exists, nil
}

func (m *Migrator) apply(ctx context.Context, file string) error {
	content, err := os.ReadFile(filepath.Join(m.dir, file))
	if err != nil {
		return fmt.Errorf("read migration %s: %w", file, err)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx %s: %w", file, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("execute migration %s: %w", file, err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, file); err != nil {
		return fmt.Errorf("save migration %s: %w", file, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", file, err)
	}

	return nil
}
