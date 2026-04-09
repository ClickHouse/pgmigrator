package main

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

func dumpSourceSchema(ctx context.Context, pgDumpPath string, src *PGConfig) (string, error) {
	filename := fmt.Sprintf("source-schema-dump-%s.sql", time.Now().Format("02_01_06_15_04_05"))

	//nolint:gosec // args are from validated config, not user shell input
	cmd := exec.CommandContext(ctx, pgDumpPath,
		"-h", src.Hostname,
		"-p", strconv.FormatUint(uint64(src.Port), 10),
		"-U", src.Username,
		"-d", src.DBName,
		"--schema-only",
		"-f", filename,
	)
	cmd.Env = append(cmd.Environ(), "PGPASSWORD="+src.Password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pg_dump failed: %w\n%s", err, output)
	}

	return filename, nil
}
