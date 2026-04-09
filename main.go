package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/urfave/cli/v3"
)

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
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dsn := cmd.String("dsn")
			if dsn == "" {
				return fmt.Errorf("dsn is required")
			}

			conn, err := pgx.Connect(ctx, dsn)
			if err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer conn.Close(ctx)

			fmt.Println("connected successfully")
			return nil
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
