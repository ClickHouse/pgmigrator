package migrate

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
)

func loadSchemaToTarget(ctx context.Context, psqlPath, schemaFile string, tgt *PGConfig) error {
	//nolint:gosec // args are from validated config, not user shell input
	cmd := exec.CommandContext(ctx, psqlPath,
		"-h", tgt.Hostname,
		"-p", strconv.FormatUint(uint64(tgt.Port), 10),
		"-U", tgt.Username,
		"-d", tgt.DBName,
		"-f", schemaFile,
	)
	cmd.Env = append(cmd.Environ(), "PGPASSWORD="+tgt.Password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql failed: %w\n%s", err, output)
	}

	return nil
}
