-- Complex schema for testing unique constraint/index backup.
-- Exercises: multiple schemas, single/multi-column unique constraints,
-- standalone unique indexes (partial, expression-based), deferrable
-- constraints, and cross-table FK references.

CREATE SCHEMA inventory;
CREATE SCHEMA analytics;

-- Simple unique constraint (single column).
CREATE TABLE public.users (
    id    SERIAL PRIMARY KEY,
    email TEXT   NOT NULL,
    username TEXT NOT NULL,
    CONSTRAINT users_email_key UNIQUE (email)
);

-- Multi-column unique constraint.
CREATE TABLE public.orders (
    id           SERIAL PRIMARY KEY,
    order_number TEXT   NOT NULL,
    region       TEXT   NOT NULL,
    CONSTRAINT orders_number_region_key UNIQUE (order_number, region)
);

-- Standalone unique index (no constraint).
CREATE TABLE public.products (
    id   SERIAL PRIMARY KEY,
    sku  TEXT   NOT NULL,
    name TEXT   NOT NULL
);
CREATE UNIQUE INDEX idx_products_sku ON public.products (sku);

-- Partial unique index (WHERE clause).
CREATE TABLE public.accounts (
    id         SERIAL PRIMARY KEY,
    email      TEXT   NOT NULL,
    deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_accounts_email_active ON public.accounts (email) WHERE deleted_at IS NULL;

-- Expression-based unique index.
CREATE TABLE public.tags (
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_tags_name_lower ON public.tags (lower(name));

-- Deferrable unique constraint.
CREATE TABLE public.positions (
    id   SERIAL PRIMARY KEY,
    rank INT    NOT NULL,
    CONSTRAINT positions_rank_key UNIQUE (rank) DEFERRABLE INITIALLY DEFERRED
);

-- Non-public schema: constraint + standalone index.
CREATE TABLE inventory.items (
    id         SERIAL PRIMARY KEY,
    barcode    TEXT   NOT NULL,
    lot_number TEXT   NOT NULL,
    CONSTRAINT items_barcode_key UNIQUE (barcode)
);
CREATE UNIQUE INDEX idx_items_lot ON inventory.items (lot_number);

-- FK referencing a PK (not a unique constraint) so it doesn't
-- block dropping the unique constraints during round-trip tests.
CREATE TABLE public.user_profiles (
    id      SERIAL PRIMARY KEY,
    user_id INT    NOT NULL REFERENCES public.users (id),
    bio     TEXT
);

-- Another schema: constraint + standalone index.
CREATE TABLE analytics.events (
    id         SERIAL PRIMARY KEY,
    event_id   UUID   NOT NULL,
    session_id TEXT   NOT NULL,
    CONSTRAINT events_event_id_key UNIQUE (event_id)
);
CREATE UNIQUE INDEX idx_events_session ON analytics.events (session_id);
