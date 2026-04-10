package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/term"
)

type PGConfig struct {
	Hostname string `toml:"hostname" json:"hostname"`
	Port     uint16 `toml:"port"     json:"port"`
	Username string `toml:"username" json:"username"`
	Password string `toml:"password" json:"password"`
	DBName   string `toml:"dbname"   json:"dbname"`
}

type Config struct {
	Source PGConfig `toml:"source" json:"source"`
	Target PGConfig `toml:"target" json:"target"`
}

func loadConfig(path string) (*Config, error) {
	var cfg Config

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

	if err := validatePGConfig("source", &cfg.Source); err != nil {
		return nil, err
	}

	if err := validatePGConfig("target", &cfg.Target); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validatePGConfig(name string, cfg *PGConfig) error {
	if cfg.Hostname == "" {
		return fmt.Errorf("[%s] hostname is required", name)
	}

	if cfg.Username == "" {
		return fmt.Errorf("[%s] username is required", name)
	}

	if cfg.DBName == "" {
		return fmt.Errorf("[%s] dbname is required", name)
	}

	if cfg.Port == 0 {
		cfg.Port = 5432
	}

	return nil
}

func promptMissingPasswords(cfg *Config) error {
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

func (p *PGConfig) ConnConfig() *pgconn.Config {
	return &pgconn.Config{
		Host:     p.Hostname,
		Port:     p.Port,
		User:     p.Username,
		Password: p.Password,
		Database: p.DBName,
	}
}

func (p *PGConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s/%s",
		p.Username,
		p.Password,
		net.JoinHostPort(p.Hostname, strconv.FormatUint(uint64(p.Port), 10)),
		p.DBName,
	)
}
