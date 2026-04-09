package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/jackc/pgx/v5"
	"github.com/urfave/cli/v3"
)

func findPgDump(path string) (string, error) {
	if path != "" {
		if _, err := exec.LookPath(path); err != nil {
			return "", fmt.Errorf("pg_dump not found at %q: %w", path, err)
		}

		return path, nil
	}

	resolved, err := exec.LookPath("pg_dump")
	if err != nil {
		return "", errors.New("pg_dump not found in PATH; install it or pass --pg-dump /path/to/pg_dump")
	}

	return resolved, nil
}

func main() {
	app := &cli.Command{
		Name:  "pgmigrator",
		Usage: "PostgreSQL migration tool",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "dsn",
				Usage:   "PostgreSQL connection string",
				Sources: cli.EnvVars("PG_DSN"),
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

			dsn := cmd.String("dsn")
			if dsn == "" {
				return errors.New("dsn is required")
			}

			conn, err := pgx.Connect(ctx, dsn)
			if err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer conn.Close(ctx)

			log.Println("connected successfully")
			return nil
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
