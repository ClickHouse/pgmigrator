package main

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	expectedUniqueConstraints       = 5
	expectedStandaloneUniqueIndexes = 5
)

func pgConfigFromContainer(ctx context.Context, t *testing.T, ctr *postgres.PostgresContainer) *PGConfig {
	t.Helper()

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	u, err := url.Parse(connStr)
	require.NoError(t, err)

	port, err := strconv.ParseUint(u.Port(), 10, 16)
	require.NoError(t, err)

	pass, _ := u.User.Password()

	return &PGConfig{
		Hostname: u.Hostname(),
		Port:     uint16(port),
		Username: u.User.Username(),
		Password: pass,
		DBName:   strings.TrimPrefix(u.Path, "/"),
	}
}

func countUniqueConstraints(ctx context.Context, t *testing.T, conn *pgx.Conn) int {
	t.Helper()

	var count int

	err := conn.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_constraint con
		JOIN pg_namespace n ON n.oid = (SELECT relnamespace FROM pg_class WHERE oid = con.conrelid)
		WHERE con.contype = 'u'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
	`).Scan(&count)
	require.NoError(t, err)

	return count
}

func countStandaloneUniqueIndexes(ctx context.Context, t *testing.T, conn *pgx.Conn) int {
	t.Helper()

	var count int

	err := conn.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_index idx
		JOIN pg_class i ON i.oid = idx.indexrelid
		JOIN pg_namespace n ON n.oid = i.relnamespace
		WHERE idx.indisunique = TRUE
		  AND NOT idx.indisprimary
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND NOT EXISTS (
		      SELECT 1 FROM pg_constraint con WHERE con.conindid = idx.indexrelid
		  )
	`).Scan(&count)
	require.NoError(t, err)

	return count
}

func TestBackupUniqueConstraints(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping integration test")
	}

	versions := []string{"16-alpine", "17-alpine", "18-alpine"}

	for _, version := range versions {
		t.Run("postgres-"+version, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			ctr, err := postgres.Run(ctx,
				"postgres:"+version,
				postgres.WithInitScripts(filepath.Join("testdata", "schema.sql")),
				postgres.WithDatabase("testdb"),
				postgres.WithUsername("testuser"),
				postgres.WithPassword("testpass"),
				postgres.BasicWaitStrategies(),
			)
			testcontainers.CleanupContainer(t, ctr)
			require.NoError(t, err)

			cfg := pgConfigFromContainer(ctx, t, ctr)
			log := zerolog.Nop()

			// Run backup.
			result, err := BackupUniqueConstraints(ctx, log, cfg)
			require.NoError(t, err)
			require.NotNil(t, result)
			t.Cleanup(func() {
				os.Remove(result.DropFile)
				os.Remove(result.RestoreFile)
			})

			dropSQL := readFile(t, result.DropFile)
			restoreSQL := readFile(t, result.RestoreFile)

			assertDropFile(t, dropSQL)
			assertRestoreFile(t, restoreSQL)
			assertRoundTrip(ctx, t, cfg, dropSQL, restoreSQL)
		})
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}

func assertDropFile(t *testing.T, sql string) {
	t.Helper()

	// All five unique constraints should have DROP CONSTRAINT statements.
	assert.Contains(t, sql, `DROP CONSTRAINT IF EXISTS "users_email_key"`)
	assert.Contains(t, sql, `DROP CONSTRAINT IF EXISTS "orders_number_region_key"`)
	assert.Contains(t, sql, `DROP CONSTRAINT IF EXISTS "positions_rank_key"`)
	assert.Contains(t, sql, `DROP CONSTRAINT IF EXISTS "items_barcode_key"`)
	assert.Contains(t, sql, `DROP CONSTRAINT IF EXISTS "events_event_id_key"`)

	// All five standalone indexes should have DROP INDEX statements.
	assert.Contains(t, sql, "idx_products_sku")
	assert.Contains(t, sql, "idx_accounts_email_active")
	assert.Contains(t, sql, "idx_tags_name_lower")
	assert.Contains(t, sql, "idx_items_lot")
	assert.Contains(t, sql, "idx_events_session")
}

func assertRestoreFile(t *testing.T, sql string) {
	t.Helper()

	// Indexes are recreated via CREATE UNIQUE INDEX.
	assert.Contains(t, sql, "CREATE UNIQUE INDEX")

	// Constraints are re-attached via USING INDEX.
	assert.Contains(t, sql, "UNIQUE USING INDEX")

	// Deferrable flag is preserved.
	assert.Contains(t, sql, "DEFERRABLE INITIALLY DEFERRED")
}

func assertRoundTrip(ctx context.Context, t *testing.T, cfg *PGConfig, dropSQL, restoreSQL string) {
	t.Helper()

	conn, err := pgx.Connect(ctx, cfg.DSN())
	require.NoError(t, err)
	defer conn.Close(ctx)

	// Verify initial counts match expectations.
	assert.Equal(t, expectedUniqueConstraints, countUniqueConstraints(ctx, t, conn))
	assert.Equal(t, expectedStandaloneUniqueIndexes, countStandaloneUniqueIndexes(ctx, t, conn))

	// Execute drop file.
	_, err = conn.Exec(ctx, dropSQL)
	require.NoError(t, err)

	// Everything should be gone.
	assert.Equal(t, 0, countUniqueConstraints(ctx, t, conn))
	assert.Equal(t, 0, countStandaloneUniqueIndexes(ctx, t, conn))

	// Execute restore file.
	_, err = conn.Exec(ctx, restoreSQL)
	require.NoError(t, err)

	// Everything should be back.
	assert.Equal(t, expectedUniqueConstraints, countUniqueConstraints(ctx, t, conn))
	assert.Equal(t, expectedStandaloneUniqueIndexes, countStandaloneUniqueIndexes(ctx, t, conn))
}
