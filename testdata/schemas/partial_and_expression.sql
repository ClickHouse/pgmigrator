-- Schema heavy on partial and expression-based unique indexes.
-- Tests edge cases: WHERE clauses, function calls, casts, multi-column expressions.

CREATE TABLE soft_deletable_users (
    id         SERIAL PRIMARY KEY,
    email      TEXT NOT NULL,
    phone      TEXT,
    deleted_at TIMESTAMPTZ
);

-- Partial: only enforce uniqueness on active rows.
CREATE UNIQUE INDEX idx_users_email_active ON soft_deletable_users (email) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_users_phone_active ON soft_deletable_users (phone) WHERE deleted_at IS NULL AND phone IS NOT NULL;

CREATE TABLE case_insensitive_lookup (
    id    SERIAL PRIMARY KEY,
    code  TEXT NOT NULL,
    label TEXT NOT NULL,
    CONSTRAINT lookup_code_key UNIQUE (code)
);

-- Expression: case-insensitive uniqueness.
CREATE UNIQUE INDEX idx_lookup_label_lower ON case_insensitive_lookup (lower(label));

CREATE TABLE composite_expressions (
    id      SERIAL PRIMARY KEY,
    region  TEXT NOT NULL,
    sku     TEXT NOT NULL,
    variant TEXT NOT NULL
);

-- Multi-column expression index.
CREATE UNIQUE INDEX idx_composite_region_sku ON composite_expressions (lower(region), upper(sku));

-- Plain multi-column unique constraint for contrast.
CREATE TABLE warehouses (
    id   SERIAL PRIMARY KEY,
    code TEXT NOT NULL,
    zone TEXT NOT NULL,
    CONSTRAINT warehouses_code_zone_key UNIQUE (code, zone)
);
