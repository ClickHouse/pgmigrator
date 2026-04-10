# pgmigrator

A PostgreSQL schema migration tool that dumps the source database schema and loads it into a target database, with built-in support for backing up unique constraints and indexes for CDC workflows.

## Why

When running Change Data Capture (CDC) with an ELT model, unique constraints and indexes on the target database cause failures due to out-of-order or duplicate data. pgmigrator automates the schema transfer and generates ready-to-use SQL files to drop and later restore unique constraints and indexes.

## Install

```
curl -fsSL https://raw.githubusercontent.com/ClickHouse/pgmigrator/main/install.sh | sh
```

This downloads the latest release for your OS/architecture, verifies the SHA256 checksum, and installs the binary to `~/.local/bin/pgmigrator`.

## Usage

```
pgmigrator -c config.toml
```

This will:

1. Dump the source schema via `pg_dump`
2. Load it into the target via `psql`
3. Connect to the target via pgx and generate two SQL files:
   - `drop-unique-<timestamp>.sql` -- run before CDC to remove unique constraints/indexes
   - `restore-unique-<timestamp>.sql` -- run after CDC to recreate them

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-c, --config` | *(required)* | Path to TOML or JSON config file |
| `--pg-dump` | search `$PATH` | Path to `pg_dump` binary |
| `--psql` | search `$PATH` | Path to `psql` binary |
| `--backup-unique` | `true` | Back up unique constraints and indexes to SQL files |

### Config file

```toml
[source]
hostname = "source-db.example.com"
port = 5432
username = "replicator"
password = "" # omit to be prompted interactively
dbname = "production"

[target]
hostname = "target-db.example.com"
port = 5432
username = "admin"
dbname = "staging"
```

JSON format is also supported (use a `.json` extension).

## Unique constraint backup

pgmigrator distinguishes between:

- **Unique constraints** (backed by an auto-created index) -- dropped via `ALTER TABLE ... DROP CONSTRAINT`, which also removes the backing index
- **Standalone unique indexes** (not backing any constraint) -- dropped via `DROP INDEX`

The generated files handle the correct dependency order:

**Drop file** (run before CDC):
```sql
BEGIN;
-- Constraints first (auto-drops backing indexes)
ALTER TABLE public.users DROP CONSTRAINT IF EXISTS "users_email_key";
-- Then standalone indexes
DROP INDEX IF EXISTS public.idx_products_sku;
COMMIT;
```

**Restore file** (run after CDC):
```sql
-- Indexes first, outside a transaction (add CONCURRENTLY if needed)
CREATE UNIQUE INDEX users_email_key ON public.users USING btree (email);
CREATE UNIQUE INDEX idx_products_sku ON public.products USING btree (sku);

-- Then re-attach constraints
BEGIN;
ALTER TABLE public.users ADD CONSTRAINT "users_email_key" UNIQUE USING INDEX "users_email_key";
COMMIT;
```

Partial indexes, expression indexes, multi-column indexes, and `DEFERRABLE` constraints are all preserved.

## Building

Requires Go 1.25+ and `pg_dump`/`psql` on the host.

```
make build    # builds bin/pgmigrator
make lint     # runs golangci-lint
make test     # runs tests (requires Docker for testcontainers)
```

## Testing

Integration tests use [testcontainers-go](https://github.com/testcontainers/testcontainers-go) to run against PostgreSQL 16, 17, and 18. Tests auto-discover schema files under `testdata/schemas/` and run each against all three versions in parallel.

To add a new test case, drop a `.sql` file into `testdata/schemas/` -- no code changes needed. The test framework captures baseline counts, executes the drop file, verifies everything is gone, executes the restore file, and verifies everything is back.

```
go test -v ./...           # run all tests
go test -short ./...       # skip integration tests
```
