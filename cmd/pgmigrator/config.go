package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/ClickHouse/pgmigrator/migrate"
	"golang.org/x/term"
)

// fileConfig is the on-disk representation of a pgmigrator config file.
type fileConfig struct {
	Source migrate.PGConfig `toml:"source" json:"source"`
	Target migrate.PGConfig `toml:"target" json:"target"`
}

func loadConfig(path string) (*fileConfig, error) {
	var cfg fileConfig

	switch filepath.Ext(path) {
	case ".toml":
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			return nil, fmt.Errorf("reading config %q: %w", path, err)
		}
	case ".json":
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("reading config %q: %w", path, readErr)
		}

		if unmarshalErr := json.Unmarshal(data, &cfg); unmarshalErr != nil {
			return nil, fmt.Errorf("parsing config %q: %w", path, unmarshalErr)
		}
	default:
		return nil, fmt.Errorf("unsupported config format %q: use .toml or .json", path)
	}

	return &cfg, nil
}

func promptMissingPasswords(cfg *fileConfig) error {
	if cfg.Source.Password == "" {
		p, err := promptPassword("source")
		if err != nil {
			return err
		}

		cfg.Source.Password = p
	}

	if cfg.Target.Password == "" {
		p, err := promptPassword("target")
		if err != nil {
			return err
		}

		cfg.Target.Password = p
	}

	return nil
}

func promptPassword(label string) (string, error) {
	fmt.Fprintf(os.Stderr, "Enter password for %s: ", label)

	password, err := term.ReadPassword(int(os.Stdin.Fd())) //nolint:gosec // stdin fd fits in int

	fmt.Fprintln(os.Stderr)

	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}

	return string(password), nil
}
