package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/ClickHouse/pgmigrator/migrate"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

var version = "dev"
var commit = "unknown" //nolint:gochecknoglobals // set by -ldflags at build time

func run() error {
	var log zerolog.Logger

	app := &cli.Command{
		Name:    "pgmigrator",
		Usage:   "PostgreSQL migration tool",
		Version: version + " (" + commit + ")",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "log-file",
				Usage: "path to log file (if not set, logs only to stderr)",
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr}
			logFilePath := cmd.String("log-file")
			if logFilePath != "" {
				logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
				if err != nil {
					return ctx, fmt.Errorf("opening log file: %w", err)
				}
				multi := io.MultiWriter(consoleWriter, logFile)
				log = zerolog.New(multi).With().Timestamp().Logger()
			} else {
				log = zerolog.New(consoleWriter).With().Timestamp().Logger()
			}
			return ctx, nil
		},
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "generate an empty config TOML file",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "output",
						Usage:   "output file path",
						Value:   "pgmigrator.toml",
						Aliases: []string{"o"},
					},
				},
				Action: func(_ context.Context, cmd *cli.Command) error {
					return generateConfig(cmd.String("output"))
				},
			},
			{
				Name:  "migrate",
				Usage: "run the migration",
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
						Value: false,
						Usage: "backup unique constraints and indexes from target to a SQL file",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg, err := loadConfig(cmd.String("config"))
					if err != nil {
						return err
					}

					if promptErr := promptMissingPasswords(cfg); promptErr != nil {
						return promptErr
					}

					_, runErr := migrate.Run(ctx, migrate.Options{
						Source:       cfg.Source,
						Target:       cfg.Target,
						PgDumpPath:   cmd.String("pg-dump"),
						PsqlPath:     cmd.String("psql"),
						OutputDir:    ".",
						BackupUnique: cmd.Bool("backup-unique"),
						Logger:       log,
					})
					return runErr
				},
			},
		},
	}

	return app.Run(context.Background(), os.Args)
}

const configTemplate = `[source]
hostname = ""
port = 5432
username = ""
password = ""
dbname = ""

[target]
hostname = ""
port = 5432
username = ""
password = ""
dbname = ""
`

func generateConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file %q already exists", path)
	}

	if err := os.WriteFile(path, []byte(configTemplate), 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "wrote %s\n", path)

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
