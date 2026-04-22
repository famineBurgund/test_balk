package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) handleDisloc(endpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetURL := strings.TrimRight(h.cfg.BaseSOAPServer, "/") + "/disloc/" + endpoint
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, targetURL, nil)
		if err != nil {
			h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")

		resp, err := h.client.Do(req)
		if err != nil {
			h.writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		defer resp.Body.Close()

		payload, err := io.ReadAll(resp.Body)
		if err != nil {
			h.writeJSON(w, http.StatusBadGateway, map[string]any{"error": "unable to read remote response"})
			return
		}
		if resp.StatusCode >= 400 {
			writeProxyResponse(w, resp.StatusCode, payload)
			return
		}

		if err := h.saveDislocFile(endpoint, payload); err != nil {
			h.logger.Warn("save disloc file failed: " + err.Error())
		}
		if err := h.persistDislocPayload(r.Context(), endpoint, payload); err != nil {
			h.logger.Warn("persist disloc payload failed: " + err.Error())
		}
		writeProxyResponse(w, http.StatusOK, payload)
	}
}

func (h *Handler) saveDislocFile(endpoint string, payload []byte) error {
	now := time.Now().UTC().Format("20060102_150405")
	latestPath := filepath.Join(h.cfg.JSONSaveDir, sanitizeFilename(endpoint)+".json")
	if err := os.WriteFile(latestPath, payload, 0o644); err != nil {
		return err
	}
	if prefix := dislocHistoricalPrefix(endpoint); prefix != "" {
		historyPath := filepath.Join(h.cfg.DislocSourceDir, fmt.Sprintf("%s_%s.json", prefix, now))
		if err := os.WriteFile(historyPath, payload, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) persistDislocPayload(ctx context.Context, endpoint string, payload []byte) error {
	var root any
	if err := json.Unmarshal(cleanJSON(payload), &root); err != nil {
		return err
	}
	records := extractRecords(root)
	source := sourceFromEndpoint(endpoint)
	switch endpoint {
	case "attis", "nmtp":
		return h.insertDislocationRows(ctx, "dislocation_main_events", endpoint, source, records)
	case "ut_emd", "gut_emd", "at_emd":
		return h.insertDislocationRows(ctx, "dislocation_emd_events", endpoint, source, records)
	case "filed-cars-at", "filed-cars-nmtp":
		return h.applyFiledCars(ctx, source, records)
	case "pps-status-at", "pps-status-nmtp":
		return h.applyPPSStatus(ctx, source, records)
	default:
		return nil
	}
}

func (h *Handler) insertDislocationRows(ctx context.Context, table, endpoint, source string, records []map[string]any) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, item := range records {
		row := makeDislocRecord(item)
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

func (h *Handler) applyFiledCars(ctx context.Context, source string, records []map[string]any) error {
	for _, item := range records {
		wagon := findString(item, wagonAliases)
		if wagon == "" {
			continue
		}
		invoice := findString(item, invoiceAliases)
		ppsNumber := findString(item, ppsAliases)
		filedAt := firstTime(item, filedAliases, eventAliases)
		if filedAt == nil {
			now := time.Now().UTC()
			filedAt = &now
		}
		payload, _ := json.Marshal(item)

		var existingID int64
		var existingFiled sql.NullTime
		err := h.db.QueryRow(ctx,
			`SELECT id, filed_at FROM wagon_visits
			 WHERE wagon_number = $1 AND source = $2 AND cleaned_at IS NULL
			 ORDER BY COALESCE(filed_at, created_at) DESC LIMIT 1`,
			wagon, source,
		).Scan(&existingID, &existingFiled)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		if err == nil {
			_, err = h.db.Exec(ctx,
				`UPDATE wagon_visits
				 SET invoice_number = COALESCE($2, invoice_number),
				     pps_number = COALESCE($3, pps_number),
				     filed_at = COALESCE(filed_at, $4),
				     opened_payload = COALESCE(opened_payload, $5),
				     updated_at = NOW()
				 WHERE id = $1`,
				existingID, nullIfEmpty(invoice), nullIfEmpty(ppsNumber), filedAt, payload,
			)
			if err != nil {
				return err
			}
			continue
		}

		_, err = h.db.Exec(ctx,
			`INSERT INTO wagon_visits (wagon_number, source, invoice_number, pps_number, filed_at, opened_payload)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			wagon, source, nullIfEmpty(invoice), nullIfEmpty(ppsNumber), filedAt, payload,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) applyPPSStatus(ctx context.Context, source string, records []map[string]any) error {
	for _, item := range records {
		wagon := findString(item, wagonAliases)
		if wagon == "" {
			continue
		}
		invoice := findString(item, invoiceAliases)
		ppsNumber := findString(item, ppsAliases)
		cleanedAt := firstTime(item, cleanAliases, eventAliases)
		payload, _ := json.Marshal(item)

		var visitID int64
		err := h.db.QueryRow(ctx,
			`SELECT id FROM wagon_visits
			 WHERE wagon_number = $1 AND source = $2 AND cleaned_at IS NULL
			 ORDER BY COALESCE(filed_at, created_at) DESC LIMIT 1`,
			wagon, source,
		).Scan(&visitID)
		if err == nil {
			if cleanedAt == nil {
				now := time.Now().UTC()
				cleanedAt = &now
			}
			_, err = h.db.Exec(ctx,
				`UPDATE wagon_visits
				 SET invoice_number = COALESCE($2, invoice_number),
				     pps_number = COALESCE($3, pps_number),
				     cleaned_at = COALESCE($4, cleaned_at),
				     closed_payload = $5,
				     updated_at = NOW()
				 WHERE id = $1`,
				visitID, nullIfEmpty(invoice), nullIfEmpty(ppsNumber), cleanedAt, payload,
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
		_, err = h.db.Exec(ctx,
			`INSERT INTO wagon_visits (wagon_number, source, invoice_number, pps_number, cleaned_at, closed_payload)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			wagon, source, nullIfEmpty(invoice), nullIfEmpty(ppsNumber), cleanedAt, payload,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
