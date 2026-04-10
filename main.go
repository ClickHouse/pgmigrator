package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

var version = "dev"
var commit = "unknown" //nolint:gochecknoglobals // set by -ldflags at build time

func run() error {
	logFile, logErr := os.OpenFile("pgmigrator.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if logErr != nil {
		return fmt.Errorf("opening log file: %w", logErr)
	}
	defer logFile.Close()

	consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr}
	multi := io.MultiWriter(consoleWriter, logFile)
	log := zerolog.New(multi).With().Timestamp().Logger()

	app := &cli.Command{
		Name:    "pgmigrator",
		Usage:   "PostgreSQL migration tool",
		Version: version + " (" + commit + ")",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Usage:    "path to TOML config file",
				Required: true,
				Aliases:  []string{"c"},
			},
			&cli.StringFlag{
				Name:  "pg-dump",
				Usage: "path to pg_dump binary (default: search PATH)",
			},
			&cli.StringFlag{
				Name:  "psql",
				Usage: "path to psql binary (default: search PATH)",
			},
			&cli.BoolFlag{
				Name:  "backup-unique",
				Value: true,
				Usage: "backup unique constraints and indexes from target to a SQL file",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return migrate(ctx, log, cmd)
		},
	}

	return app.Run(context.Background(), os.Args)
}

func migrate(ctx context.Context, log zerolog.Logger, cmd *cli.Command) error {
	pgDump, err := findBinary("pg_dump", cmd.String("pg-dump"))
	if err != nil {
		return err
	}

	psql, err := findBinary("psql", cmd.String("psql"))
	if err != nil {
		return err
	}

	log.Info().Str("pg_dump", pgDump).Str("psql", psql).Msg("found binaries")

	cfg, err := loadConfig(cmd.String("config"))
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

	if cmd.Bool("backup-unique") {
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

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
