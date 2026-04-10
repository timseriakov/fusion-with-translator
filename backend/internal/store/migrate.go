package store

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Migration files must follow naming convention: NNN_description.sql
// where NNN is a zero-padded version number (e.g., 001_initial.sql).
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// goMigrations defines code-only migrations that run after SQL migrations.
// Used when a migration requires Go logic (e.g., data transformation).
var goMigrations = []struct {
	version int
	name    string
	migrate func(db *sql.DB) error
}{
	{version: 6, name: "convert_html_to_markdown", migrate: convertHTMLToMarkdown},
}

func (s *Store) migrate() error {
	startedAt := time.Now()
	slog.Info("database migration started")

	if err := s.createMigrationsTable(); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	applied, err := s.getAppliedVersions()
	if err != nil {
		return fmt.Errorf("get applied versions: %w", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	appliedCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		version, err := extractVersion(entry.Name())
		if err != nil {
			return fmt.Errorf("invalid migration filename %s: %w", entry.Name(), err)
		}

		if applied[version] {
			slog.Debug("migration already applied", "version", version, "file", entry.Name())
			continue
		}

		slog.Info("applying migration", "version", version, "file", entry.Name())
		if err := s.applyMigration(version, entry.Name()); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}

		appliedCount++
		slog.Info("migration applied", "version", version, "file", entry.Name())
	}

	// Apply Go-based migrations.
	for _, gm := range goMigrations {
		if applied[gm.version] {
			slog.Debug("Go migration already applied", "version", gm.version, "name", gm.name)
			continue
		}

		slog.Info("applying Go migration", "version", gm.version, "name", gm.name)
		if err := gm.migrate(s.db); err != nil {
			return fmt.Errorf("apply Go migration %s: %w", gm.name, err)
		}

		if _, err := s.db.Exec(
			"INSERT INTO schema_migrations (version) VALUES (:version)",
			sql.Named("version", gm.version),
		); err != nil {
			return fmt.Errorf("record Go migration version: %w", err)
		}

		appliedCount++
		slog.Info("Go migration applied", "version", gm.version, "name", gm.name)
	}

	slog.Info(
		"database migration finished",
		"applied", appliedCount,
		"duration", time.Since(startedAt),
	)

	return nil
}

func (s *Store) createMigrationsTable() error {
	schema := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL DEFAULT (unixepoch())
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) getAppliedVersions() (map[int]bool, error) {
	rows, err := s.db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// applyMigration executes a migration file within a transaction.
// Both the migration SQL and version record are committed atomically,
// ensuring consistent migration state even if the process crashes.
func (s *Store) applyMigration(version int, filename string) error {
	content, err := migrationFiles.ReadFile("migrations/" + filename)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(string(content)); err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}

	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version) VALUES (:version)",
		sql.Named("version", version),
	); err != nil {
		return fmt.Errorf("record version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func extractVersion(filename string) (int, error) {
	if !strings.HasSuffix(filename, ".sql") {
		return 0, fmt.Errorf("not a .sql file")
	}

	parts := strings.SplitN(filename, "_", 2)
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid format, expected NNN_description.sql")
	}

	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid version number: %w", err)
	}

	return version, nil
}
