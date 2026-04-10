package migrate

import (
	"context"

	"github.com/rs/zerolog"
)

// Options holds the CLI-parsed parameters for a migration run.
type Options struct {
	ConfigPath   string
	PgDumpPath   string
	PsqlPath     string
	BackupUnique bool
}

// Run executes the full migration workflow.
func Run(ctx context.Context, log zerolog.Logger, opts Options) error {
	pgDump, err := findBinary("pg_dump", opts.PgDumpPath)
	if err != nil {
		return err
	}

	psql, err := findBinary("psql", opts.PsqlPath)
	if err != nil {
		return err
	}

	log.Info().Str("pg_dump", pgDump).Str("psql", psql).Msg("found binaries")

	cfg, err := loadConfig(opts.ConfigPath)
	if err != nil {
		return err
	}

	if promptErr := promptMissingPasswords(cfg); promptErr != nil {
		return promptErr
	}

	schemaFile, err := dumpSourceSchema(ctx, pgDump, &cfg.Source)
	if err != nil {
		return err
	}

	log.Info().Str("file", schemaFile).Msg("source schema dumped")

	if loadErr := loadSchemaToTarget(ctx, psql, schemaFile, &cfg.Target); loadErr != nil {
		return loadErr
	}

	log.Info().Msg("schema loaded to target")

	if opts.BackupUnique {
		result, backupErr := BackupUniqueConstraints(ctx, log, &cfg.Target, ".")
		if backupErr != nil {
			return backupErr
		}

		if result.DropFile != "" {
			log.Info().
				Str("drop", result.DropFile).
				Str("restore", result.RestoreFile).
				Msg("unique constraints backed up")
		}
	}

	return nil
}
