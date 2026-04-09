package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/urfave/cli/v3"
)

func main() {
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

			log.Printf("using pg_dump: %s", pgDump)

			cfg, err := loadConfig(cmd.String("config"))
			if err != nil {
				return err
			}

			if promptErr := promptMissingPasswords(cfg); promptErr != nil {
				return promptErr
			}

			sourcePool, err := pgxpool.New(ctx, cfg.Source.DSN())
			if err != nil {
				return fmt.Errorf("connect to source: %w", err)
			}
			defer sourcePool.Close()

			targetPool, err := pgxpool.New(ctx, cfg.Target.DSN())
			if err != nil {
				return fmt.Errorf("connect to target: %w", err)
			}
			defer targetPool.Close()

			log.Println("connected to source and target")
			return nil
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
