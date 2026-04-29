package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"dislocservice/internal/parser/dislocjson"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Repository struct {
	db DB
}

type WagonVisit struct {
	ID            int64      `json:"id"`
	WagonNumber   string     `json:"wagon_number"`
	Source        string     `json:"source"`
	InvoiceNum    string     `json:"invoice_number,omitempty"`
	PPSNumber     string     `json:"pps_number,omitempty"`
	FiledAt       *time.Time `json:"filed_at,omitempty"`
	CleanedAt     *time.Time `json:"cleaned_at,omitempty"`
	OpenedPayload []byte     `json:"opened_payload,omitempty"`
	ClosedPayload []byte     `json:"closed_payload,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func New(db DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListInvoiceBatchPayloads(ctx context.Context, direction string) ([][]byte, error) {
	rows, err := r.db.Query(ctx, `SELECT payload FROM invoice_batches WHERE direction = $1 ORDER BY created_at DESC`, direction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payloads [][]byte
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		payloads = append(payloads, payload)
	}
	return payloads, rows.Err()
}

func (r *Repository) InsertInvoiceBatch(ctx context.Context, direction string, payload []byte) error {
	_, err := r.db.Exec(ctx, `INSERT INTO invoice_batches (direction, payload) VALUES ($1, $2)`, direction, payload)
	return err
}

func (r *Repository) InsertDislocationRows(ctx context.Context, table, endpoint, source string, records []dislocjson.DislocRecord) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, row := range records {
		if row.WagonNumber == "" {
			continue
		}
		_, err := tx.Exec(ctx,
			fmt.Sprintf(`INSERT INTO %s (wagon_number, source, endpoint, invoice_number, event_time, payload) VALUES ($1, $2, $3, $4, $5, $6)`, table),
			row.WagonNumber, source, endpoint, nullIfEmpty(row.InvoiceNum), row.EventTime, row.Payload,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) ApplyFiledCars(ctx context.Context, source string, records []dislocjson.WagonEvent) error {
	for _, item := range records {
		if item.WagonNumber == "" {
			continue
		}
		filedAt := item.FiledAt
		if filedAt == nil {
			now := time.Now().UTC()
			filedAt = &now
		}

		var existingID int64
		var existingFiled sql.NullTime
		err := r.db.QueryRow(ctx,
			`SELECT id, filed_at FROM wagon_visits
			 WHERE wagon_number = $1 AND source = $2 AND cleaned_at IS NULL
			 ORDER BY COALESCE(filed_at, created_at) DESC LIMIT 1`,
			item.WagonNumber, source,
		).Scan(&existingID, &existingFiled)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		if err == nil {
			_, err = r.db.Exec(ctx,
				`UPDATE wagon_visits
				 SET invoice_number = COALESCE($2, invoice_number),
				     pps_number = COALESCE($3, pps_number),
				     filed_at = COALESCE(filed_at, $4),
				     opened_payload = COALESCE(opened_payload, $5),
				     updated_at = NOW()
				 WHERE id = $1`,
				existingID, nullIfEmpty(item.InvoiceNum), nullIfEmpty(item.PPSNumber), filedAt, item.Payload,
			)
			if err != nil {
				return err
			}
			continue
		}

		_, err = r.db.Exec(ctx,
			`INSERT INTO wagon_visits (wagon_number, source, invoice_number, pps_number, filed_at, opened_payload)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			item.WagonNumber, source, nullIfEmpty(item.InvoiceNum), nullIfEmpty(item.PPSNumber), filedAt, item.Payload,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ApplyPPSStatus(ctx context.Context, source string, records []dislocjson.WagonEvent) error {
	for _, item := range records {
		if item.WagonNumber == "" {
			continue
		}
		cleanedAt := item.CleanedAt

		var visitID int64
		err := r.db.QueryRow(ctx,
			`SELECT id FROM wagon_visits
			 WHERE wagon_number = $1 AND source = $2 AND cleaned_at IS NULL
			 ORDER BY COALESCE(filed_at, created_at) DESC LIMIT 1`,
			item.WagonNumber, source,
		).Scan(&visitID)
		if err == nil {
			if cleanedAt == nil {
				now := time.Now().UTC()
				cleanedAt = &now
			}
			_, err = r.db.Exec(ctx,
				`UPDATE wagon_visits
				 SET invoice_number = COALESCE($2, invoice_number),
				     pps_number = COALESCE($3, pps_number),
				     cleaned_at = COALESCE($4, cleaned_at),
				     closed_payload = $5,
				     updated_at = NOW()
				 WHERE id = $1`,
				visitID, nullIfEmpty(item.InvoiceNum), nullIfEmpty(item.PPSNumber), cleanedAt, item.Payload,
			)
			if err != nil {
				return err
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		if cleanedAt == nil {
			now := time.Now().UTC()
			cleanedAt = &now
		}
		_, err = r.db.Exec(ctx,
			`INSERT INTO wagon_visits (wagon_number, source, invoice_number, pps_number, cleaned_at, closed_payload)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			item.WagonNumber, source, nullIfEmpty(item.InvoiceNum), nullIfEmpty(item.PPSNumber), cleanedAt, item.Payload,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ListLatestWagonVisits(ctx context.Context) ([]WagonVisit, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (wagon_number) id, wagon_number, source, invoice_number, pps_number, filed_at, cleaned_at, opened_payload, closed_payload, created_at, updated_at
		FROM wagon_visits
		ORDER BY wagon_number, COALESCE(cleaned_at, filed_at, created_at) DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVisits(rows)
}

func (r *Repository) ListFiledWagonVisits(ctx context.Context) ([]WagonVisit, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, wagon_number, source, invoice_number, pps_number, filed_at, cleaned_at, opened_payload, closed_payload, created_at, updated_at
		FROM wagon_visits
		WHERE cleaned_at IS NULL
		ORDER BY COALESCE(filed_at, created_at) DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVisits(rows)
}

func (r *Repository) ListCleanWagonVisits(ctx context.Context) ([]WagonVisit, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, wagon_number, source, invoice_number, pps_number, filed_at, cleaned_at, opened_payload, closed_payload, created_at, updated_at
		FROM wagon_visits
		WHERE cleaned_at IS NOT NULL
		ORDER BY COALESCE(cleaned_at, updated_at) DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVisits(rows)
}

func (r *Repository) ListWagonHistory(ctx context.Context, wagon string) ([]WagonVisit, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, wagon_number, source, invoice_number, pps_number, filed_at, cleaned_at, opened_payload, closed_payload, created_at, updated_at
		FROM wagon_visits
		WHERE wagon_number = $1
		ORDER BY COALESCE(filed_at, created_at) DESC, id DESC`, wagon)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVisits(rows)
}

type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanVisits(rows pgxRows) ([]WagonVisit, error) {
	var visits []WagonVisit
	for rows.Next() {
		var visit WagonVisit
		var invoice, pps sql.NullString
		var filedAt, cleanedAt sql.NullTime
		if err := rows.Scan(
			&visit.ID, &visit.WagonNumber, &visit.Source, &invoice, &pps, &filedAt, &cleanedAt,
			&visit.OpenedPayload, &visit.ClosedPayload, &visit.CreatedAt, &visit.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if invoice.Valid {
			visit.InvoiceNum = invoice.String
		}
		if pps.Valid {
			visit.PPSNumber = pps.String
		}
		if filedAt.Valid {
			value := filedAt.Time
			visit.FiledAt = &value
		}
		if cleanedAt.Valid {
			value := cleanedAt.Time
			visit.CleanedAt = &value
		}
		visits = append(visits, visit)
	}
	return visits, rows.Err()
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
