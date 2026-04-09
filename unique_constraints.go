package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
)

type uniqueConstraint struct {
	TableName      string
	ConstraintName string
	IndexName      string
	IndexDef       string
	Deferrable     bool
	Deferred       bool
}

type uniqueIndex struct {
	IndexName string
	IndexDef  string
}

func BackupUniqueConstraints(ctx context.Context, log zerolog.Logger, cfg *PGConfig) (string, error) {
	conn, err := pgx.Connect(ctx, cfg.DSN())
	if err != nil {
		return "", fmt.Errorf("connecting to target: %w", err)
	}
	defer conn.Close(ctx)

	constraints, err := fetchUniqueConstraints(ctx, conn)
	if err != nil {
		return "", err
	}

	indexes, err := fetchStandaloneUniqueIndexes(ctx, conn)
	if err != nil {
		return "", err
	}

	if len(constraints) == 0 && len(indexes) == 0 {
		log.Info().Msg("no unique constraints or standalone unique indexes found")
		return "", nil
	}

	log.Info().
		Int("constraints", len(constraints)).
		Int("standalone_indexes", len(indexes)).
		Msg("found unique constraints and indexes")

	filename := fmt.Sprintf("unique-constraints-backup-%s.sql", time.Now().Format("02_01_06_15_04_05"))

	if writeErr := writeBackupFile(filename, constraints, indexes); writeErr != nil {
		return "", writeErr
	}

	return filename, nil
}

func fetchUniqueConstraints(ctx context.Context, conn *pgx.Conn) ([]uniqueConstraint, error) {
	query := `
SELECT
    quote_ident(n.nspname) || '.' || quote_ident(c.relname),
    con.conname,
    i.relname,
    pg_get_indexdef(con.conindid),
    con.condeferrable,
    con.condeferred
FROM pg_constraint con
JOIN pg_class c ON c.oid = con.conrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_class i ON i.oid = con.conindid
WHERE con.contype = 'u'
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
ORDER BY n.nspname, c.relname, con.conname`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying unique constraints: %w", err)
	}
	defer rows.Close()

	var constraints []uniqueConstraint

	for rows.Next() {
		var uc uniqueConstraint
		if scanErr := rows.Scan(
			&uc.TableName, &uc.ConstraintName, &uc.IndexName,
			&uc.IndexDef, &uc.Deferrable, &uc.Deferred,
		); scanErr != nil {
			return nil, fmt.Errorf("scanning unique constraint row: %w", scanErr)
		}

		constraints = append(constraints, uc)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterating unique constraints: %w", rows.Err())
	}

	return constraints, nil
}

func fetchStandaloneUniqueIndexes(ctx context.Context, conn *pgx.Conn) ([]uniqueIndex, error) {
	query := `
SELECT
    quote_ident(n.nspname) || '.' || quote_ident(i.relname),
    pg_get_indexdef(ix.indexrelid)
FROM pg_class t
JOIN pg_index ix ON t.oid = ix.indrelid
JOIN pg_class i ON i.oid = ix.indexrelid
JOIN pg_namespace n ON n.oid = i.relnamespace
WHERE ix.indisunique = TRUE
  AND ix.indisprimary = FALSE
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
  AND NOT EXISTS (
      SELECT 1 FROM pg_constraint con
      WHERE con.conindid = ix.indexrelid
  )
ORDER BY n.nspname, i.relname`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying standalone unique indexes: %w", err)
	}
	defer rows.Close()

	var indexes []uniqueIndex

	for rows.Next() {
		var ui uniqueIndex
		if scanErr := rows.Scan(&ui.IndexName, &ui.IndexDef); scanErr != nil {
			return nil, fmt.Errorf("scanning unique index row: %w", scanErr)
		}

		indexes = append(indexes, ui)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterating unique indexes: %w", rows.Err())
	}

	return indexes, nil
}

func writeBackupFile(filename string, constraints []uniqueConstraint, indexes []uniqueIndex) error {
	var b strings.Builder

	b.WriteString("-- pgmigrator: unique constraint and index backup\n")
	fmt.Fprintf(&b, "-- Generated: %s\n", time.Now().Format(time.RFC3339))
	b.WriteString("--\n")
	b.WriteString("-- DROP order: constraints first (auto-drops backing index), then standalone indexes\n")
	b.WriteString("-- RESTORE order: all indexes first, then re-attach constraints via USING INDEX\n\n")

	writeDropSection(&b, constraints, indexes)
	writeRestoreSection(&b, constraints, indexes)

	//nolint:gosec // backup file does not need restricted permissions
	if err := os.WriteFile(filename, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("writing backup file %q: %w", filename, err)
	}

	return nil
}

func writeDropSection(b *strings.Builder, constraints []uniqueConstraint, indexes []uniqueIndex) {
	b.WriteString("-- ============================================================\n")
	b.WriteString("-- SECTION 1: DROP\n")
	b.WriteString("-- Run these statements to remove unique constraints before CDC\n")
	b.WriteString("-- ============================================================\n\n")
	b.WriteString("BEGIN;\n\n")

	if len(constraints) > 0 {
		b.WriteString("-- Unique constraints (dropping also removes their backing index)\n")

		for _, uc := range constraints {
			fmt.Fprintf(b, "ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;\n",
				uc.TableName, quoteIdent(uc.ConstraintName))
		}

		b.WriteString("\n")
	}

	if len(indexes) > 0 {
		b.WriteString("-- Standalone unique indexes (not backing any constraint)\n")

		for _, ui := range indexes {
			fmt.Fprintf(b, "DROP INDEX IF EXISTS %s;\n", ui.IndexName)
		}

		b.WriteString("\n")
	}

	b.WriteString("COMMIT;\n\n")
}

func writeRestoreSection(b *strings.Builder, constraints []uniqueConstraint, indexes []uniqueIndex) {
	b.WriteString("-- ============================================================\n")
	b.WriteString("-- SECTION 2: RESTORE\n")
	b.WriteString("-- Run these statements to recreate constraints after CDC\n")
	b.WriteString("-- ============================================================\n\n")

	// Indexes outside transaction so CONCURRENTLY can be added if desired.
	if len(constraints) > 0 || len(indexes) > 0 {
		b.WriteString("-- Recreate all unique indexes\n")
		b.WriteString("-- (not wrapped in a transaction so you can add CONCURRENTLY if needed)\n\n")

		for _, uc := range constraints {
			b.WriteString(uc.IndexDef + ";\n")
		}

		for _, ui := range indexes {
			b.WriteString(ui.IndexDef + ";\n")
		}

		b.WriteString("\n")
	}

	if len(constraints) > 0 {
		b.WriteString("-- Re-attach constraints to their backing indexes\n")
		b.WriteString("BEGIN;\n\n")

		for _, uc := range constraints {
			fmt.Fprintf(b, "ALTER TABLE %s ADD CONSTRAINT %s UNIQUE USING INDEX %s",
				uc.TableName, quoteIdent(uc.ConstraintName), quoteIdent(uc.IndexName))

			if uc.Deferrable {
				b.WriteString(" DEFERRABLE")
				if uc.Deferred {
					b.WriteString(" INITIALLY DEFERRED")
				}
			}

			b.WriteString(";\n")
		}

		b.WriteString("\nCOMMIT;\n")
	}
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
