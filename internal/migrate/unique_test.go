package migrate_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ClickHouse/pgmigrator/internal/migrate"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var pgVersions = []string{"16-alpine", "17-alpine", "18-alpine"} //nolint:gochecknoglobals // test matrix

func discoverSchemas(t *testing.T) []string {
	t.Helper()

	schemas, err := filepath.Glob(filepath.Join("testdata", "schemas", "*.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, schemas, "no schema files found in testdata/schemas/")

	return schemas
}

func pgConfigFromContainer(ctx context.Context, t *testing.T, ctr *postgres.PostgresContainer) *migrate.PGConfig {
	t.Helper()

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	u, err := url.Parse(connStr)
	require.NoError(t, err)

	port, err := strconv.ParseUint(u.Port(), 10, 16)
	require.NoError(t, err)

	pass, _ := u.User.Password()

	return &migrate.PGConfig{
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

	schemas := discoverSchemas(t)

	for _, schema := range schemas {
		for _, version := range pgVersions {
			name := filepath.Base(schema) + "/postgres-" + version

			t.Run(name, func(t *testing.T) {
				t.Parallel()
				runBackupTest(t, schema, version)
			})
		}
	}
}

func runBackupTest(t *testing.T, schemaFile, pgVersion string) {
	t.Helper()

	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:"+pgVersion,
		postgres.WithInitScripts(schemaFile),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, ctr)
	require.NoError(t, err)

	cfg := pgConfigFromContainer(ctx, t, ctr)

	// Capture baseline counts before backup.
	conn, err := pgx.Connect(ctx, cfg.DSN())
	require.NoError(t, err)

	baselineConstraints := countUniqueConstraints(ctx, t, conn)
	baselineIndexes := countStandaloneUniqueIndexes(ctx, t, conn)
	conn.Close(ctx)

	require.Positive(t, baselineConstraints+baselineIndexes,
		"schema %s has no unique constraints or indexes to test", schemaFile)

	// Run backup.
	result, err := migrate.BackupUniqueConstraints(ctx, zerolog.Nop(), cfg, t.TempDir())
	require.NoError(t, err)
	require.NotEmpty(t, result.DropFile)
	require.NotEmpty(t, result.RestoreFile)

	dropSQL := readFile(t, result.DropFile)
	restoreSQL := readFile(t, result.RestoreFile)

	assertDropSQL(t, dropSQL, baselineConstraints, baselineIndexes)
	assertRestoreSQL(t, restoreSQL, baselineConstraints)
	assertRoundTrip(ctx, t, cfg, dropSQL, restoreSQL, baselineConstraints, baselineIndexes)
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}

func assertDropSQL(t *testing.T, sql string, constraints, indexes int) {
	t.Helper()

	if constraints > 0 {
		assert.Contains(t, sql, "DROP CONSTRAINT IF EXISTS")
	}

	if indexes > 0 {
		assert.Contains(t, sql, "DROP INDEX IF EXISTS")
	}
}

func assertRestoreSQL(t *testing.T, sql string, constraints int) {
	t.Helper()

	assert.Contains(t, sql, "CREATE UNIQUE INDEX")

	if constraints > 0 {
		assert.Contains(t, sql, "UNIQUE USING INDEX")
	}
}

func assertRoundTrip(
	ctx context.Context,
	t *testing.T,
	cfg *migrate.PGConfig,
	dropSQL, restoreSQL string,
	baselineConstraints, baselineIndexes int,
) {
	t.Helper()

	conn, err := pgx.Connect(ctx, cfg.DSN())
	require.NoError(t, err)
	defer conn.Close(ctx)

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
	assert.Equal(t, baselineConstraints, countUniqueConstraints(ctx, t, conn))
	assert.Equal(t, baselineIndexes, countStandaloneUniqueIndexes(ctx, t, conn))
}
