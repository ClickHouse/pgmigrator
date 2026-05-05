package migrate

import (
	"fmt"
	"net"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
)

// PGConfig identifies a PostgreSQL database and the credentials used to
// connect to it. The toml and json struct tags allow callers to decode it
// from configuration files, but the type itself has no I/O behavior.
type PGConfig struct {
	Hostname string `toml:"hostname" json:"hostname"`
	Port     uint16 `toml:"port"     json:"port"`
	Username string `toml:"username" json:"username"`
	Password string `toml:"password" json:"password"`
	DBName   string `toml:"dbname"   json:"dbname"`
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

// ConnConfig returns a pgx connection config for p.
func (p *PGConfig) ConnConfig() *pgconn.Config {
	return &pgconn.Config{
		Host:     p.Hostname,
		Port:     p.Port,
		User:     p.Username,
		Password: p.Password,
		Database: p.DBName,
	}
}

// DSN returns a PostgreSQL connection URI for p.
func (p *PGConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s/%s",
		p.Username,
		p.Password,
		net.JoinHostPort(p.Hostname, strconv.FormatUint(uint64(p.Port), 10)),
		p.DBName,
	)
}
