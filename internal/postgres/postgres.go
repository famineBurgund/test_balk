package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func InitSchema(ctx context.Context, db *pgxpool.Pool) error {
	schema := `
CREATE TABLE IF NOT EXISTS dislocation_main_events (
	id BIGSERIAL PRIMARY KEY,
	wagon_number TEXT NOT NULL,
	source TEXT NOT NULL,
	endpoint TEXT NOT NULL,
	invoice_number TEXT,
	event_time TIMESTAMPTZ,
	payload JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dislocation_main_latest
	ON dislocation_main_events (wagon_number, created_at DESC);

CREATE TABLE IF NOT EXISTS dislocation_emd_events (
	id BIGSERIAL PRIMARY KEY,
	wagon_number TEXT NOT NULL,
	source TEXT NOT NULL,
	endpoint TEXT NOT NULL,
	invoice_number TEXT,
	event_time TIMESTAMPTZ,
	payload JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dislocation_emd_latest
	ON dislocation_emd_events (wagon_number, created_at DESC);

CREATE TABLE IF NOT EXISTS wagon_visits (
	id BIGSERIAL PRIMARY KEY,
	wagon_number TEXT NOT NULL,
	source TEXT NOT NULL,
	invoice_number TEXT,
	pps_number TEXT,
	filed_at TIMESTAMPTZ,
	cleaned_at TIMESTAMPTZ,
	opened_payload JSONB,
	closed_payload JSONB,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wagon_visits_open
	ON wagon_visits (wagon_number, source, cleaned_at);

CREATE TABLE IF NOT EXISTS invoice_batches (
	id BIGSERIAL PRIMARY KEY,
	direction TEXT NOT NULL,
	payload JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

	if _, err := db.Exec(ctx, schema); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
}
