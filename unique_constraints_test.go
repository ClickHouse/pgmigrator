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
			filename, err := BackupUniqueConstraints(ctx, log, cfg)
			require.NoError(t, err)
			require.NotEmpty(t, filename)
			t.Cleanup(func() { os.Remove(filename) })

			content, err := os.ReadFile(filename)
			require.NoError(t, err)

			sql := string(content)

			assertDropSection(t, sql)
			assertRestoreSection(t, sql)
			assertRoundTrip(ctx, t, cfg, sql)
		})
	}
}

func assertDropSection(t *testing.T, sql string) {
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

func assertRestoreSection(t *testing.T, sql string) {
	t.Helper()

	// Indexes are recreated via CREATE UNIQUE INDEX.
	assert.Contains(t, sql, "CREATE UNIQUE INDEX")

	// Constraints are re-attached via USING INDEX.
	assert.Contains(t, sql, "UNIQUE USING INDEX")

	// Deferrable flag is preserved.
	assert.Contains(t, sql, "DEFERRABLE INITIALLY DEFERRED")
}

func assertRoundTrip(ctx context.Context, t *testing.T, cfg *PGConfig, sql string) {
	t.Helper()

	conn, err := pgx.Connect(ctx, cfg.DSN())
	require.NoError(t, err)
	defer conn.Close(ctx)

	// Verify initial counts match expectations.
	assert.Equal(t, expectedUniqueConstraints, countUniqueConstraints(ctx, t, conn))
	assert.Equal(t, expectedStandaloneUniqueIndexes, countStandaloneUniqueIndexes(ctx, t, conn))

	// Execute DROP section (everything before SECTION 2).
	dropEnd := strings.Index(sql, "-- SECTION 2: RESTORE")
	require.Positive(t, dropEnd)

	_, err = conn.Exec(ctx, sql[:dropEnd])
	require.NoError(t, err)

	// Everything should be gone.
	assert.Equal(t, 0, countUniqueConstraints(ctx, t, conn))
	assert.Equal(t, 0, countStandaloneUniqueIndexes(ctx, t, conn))

	// Execute RESTORE section.
	_, err = conn.Exec(ctx, sql[dropEnd:])
	require.NoError(t, err)

	// Everything should be back.
	assert.Equal(t, expectedUniqueConstraints, countUniqueConstraints(ctx, t, conn))
	assert.Equal(t, expectedStandaloneUniqueIndexes, countStandaloneUniqueIndexes(ctx, t, conn))
}
