package main

import (
	"errors"
	"fmt"
	"os/exec"
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
