-- Minimal schema: one unique constraint, one standalone unique index.
-- Smoke test to verify the backup works on the simplest case.

CREATE TABLE tenants (
    id     SERIAL PRIMARY KEY,
    slug   TEXT NOT NULL,
    domain TEXT NOT NULL,
    CONSTRAINT tenants_slug_key UNIQUE (slug)
);

CREATE UNIQUE INDEX idx_tenants_domain ON tenants (domain);
