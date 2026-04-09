package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

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
		Name:  "pgmigrator",
		Usage: "PostgreSQL migration tool",
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
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			pgDump, err := findPgDump(cmd.String("pg-dump"))
			if err != nil {
				return err
			}

			log.Info().Str("path", pgDump).Msg("using pg_dump")

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
			return nil
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
