package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/ClickHouse/pgmigrator/internal/migrate"
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
			return migrate.Run(ctx, log, migrate.Options{
				ConfigPath:   cmd.String("config"),
				PgDumpPath:   cmd.String("pg-dump"),
				PsqlPath:     cmd.String("psql"),
				BackupUnique: cmd.Bool("backup-unique"),
			})
		},
	}

	return app.Run(context.Background(), os.Args)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
