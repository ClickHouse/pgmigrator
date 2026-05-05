// Package migrate performs a schema-only migration from a source PostgreSQL
// database to a target PostgreSQL database, optionally backing up unique
// constraints and indexes on the target prior to migration.
//
// The package is intended to be usable both as a library and from the
// pgmigrator CLI. Callers construct an [Options] value and call [Run].
package migrate

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

// Options configures a single migration run.
//
// Source and Target are required. PgDumpPath and PsqlPath are optional;
// if empty, the binaries are looked up on PATH.
type Options struct {
	// Source is the database to dump the schema from.
	Source PGConfig
	// Target is the database to load the schema into.
	Target PGConfig

	// PgDumpPath is an optional explicit path to the pg_dump binary.
	// If empty, pg_dump is resolved from PATH.
	PgDumpPath string
	// PsqlPath is an optional explicit path to the psql binary.
	// If empty, psql is resolved from PATH.
	PsqlPath string

	// OutputDir is the directory where the schema dump and (optionally)
	// the unique-constraint backup files are written. Defaults to ".".
	OutputDir string

	// BackupUnique controls whether unique constraints and standalone
	// unique indexes on the target are backed up to drop/restore SQL
	// files prior to the migration.
	BackupUnique bool

	// Logger receives structured progress logs. The zero value is a
	// disabled logger, which suppresses all output.
	Logger zerolog.Logger
}

// Result describes the artifacts produced by a successful [Run].
type Result struct {
	// SchemaFile is the path of the source-schema dump file.
	SchemaFile string
	// DropFile is the path of the SQL file that drops unique
	// constraints and indexes on the target. Empty unless
	// [Options.BackupUnique] was true and at least one was found.
	DropFile string
	// RestoreFile is the path of the SQL file that recreates unique
	// constraints and indexes on the target. Empty unless
	// [Options.BackupUnique] was true and at least one was found.
	RestoreFile string
}

// Run executes the migration described by opts.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if err := validatePGConfig("source", &opts.Source); err != nil {
		return nil, err
	}

	if err := validatePGConfig("target", &opts.Target); err != nil {
		return nil, err
	}

	if opts.OutputDir == "" {
		opts.OutputDir = "."
	}

	log := opts.Logger

	pgDump, err := findBinary("pg_dump", opts.PgDumpPath)
	if err != nil {
		return nil, err
	}

	psql, err := findBinary("psql", opts.PsqlPath)
	if err != nil {
		return nil, err
	}

	log.Info().Str("pg_dump", pgDump).Str("psql", psql).Msg("found binaries")

	schemaFile, err := dumpSourceSchema(ctx, pgDump, &opts.Source, opts.OutputDir)
	if err != nil {
		return nil, err
	}

	log.Info().Str("file", schemaFile).Msg("source schema dumped")

	if loadErr := loadSchemaToTarget(ctx, psql, schemaFile, &opts.Target); loadErr != nil {
		return nil, fmt.Errorf("loading schema to target: %w", loadErr)
	}

	log.Info().Msg("schema loaded to target")

	result := &Result{SchemaFile: schemaFile}

	if opts.BackupUnique {
		backup, backupErr := BackupUniqueConstraints(ctx, log, &opts.Target, opts.OutputDir)
		if backupErr != nil {
			return result, backupErr
		}

		if backup.DropFile != "" {
			log.Info().
				Str("drop", backup.DropFile).
				Str("restore", backup.RestoreFile).
				Msg("unique constraints backed up")
		}

		result.DropFile = backup.DropFile
		result.RestoreFile = backup.RestoreFile
	}

	return result, nil
}
