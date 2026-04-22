package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

func (h *Handler) handleLastInterval(direction string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		rows, err := h.db.Query(ctx, `SELECT payload FROM invoice_batches WHERE direction = $1 ORDER BY created_at DESC`, direction)
		if err != nil {
			h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		defer rows.Close()

		var latest *time.Time
		for rows.Next() {
			var payload []byte
			if err := rows.Scan(&payload); err == nil {
				latest = maxIntervalTo(payload, latest)
			}
		}

		if latest == nil {
			h.writeJSON(w, http.StatusOK, map[string]any{"lastInterval": nil})
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]any{"lastInterval": latest.UTC().Format(time.RFC3339)})
	}
}

func (h *Handler) handleProcessInvoices(direction string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unable to read request body"})
			return
		}
		if !json.Valid(body) {
			h.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if _, err := h.db.Exec(ctx, `INSERT INTO invoice_batches (direction, payload) VALUES ($1, $2)`, direction, body); err != nil {
			h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		target := filepath.Join(h.cfg.InvoiceStorageDir, fmt.Sprintf("last_interval_%s.json", direction))
		if err := appendJSONBatch(target, body, 50); err != nil {
			h.logger.Warn("save invoice batch file failed: " + err.Error())
		}
		h.writeJSON(w, http.StatusOK, map[string]any{"status": "success"})
	}
}

func (h *Handler) handleInvoiceSearch(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		InvoiceNumber string `json:"invoice_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if !isTenDigits(payload.InvoiceNumber) {
		h.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invoice_number must contain 10 digits"})
		return
	}
	h.proxyJSONPost(w, r, "/invoice/search", map[string]string{"invoice_number": payload.InvoiceNumber})
}

func (h *Handler) handleInvoiceIDSearch(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		InvoiceID string `json:"invoice_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(payload.InvoiceID) == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invoice_id is required"})
		return
	}
	h.proxyJSONPost(w, r, "/invoice/idsearch", map[string]string{"invoice_id": payload.InvoiceID})
}

func (h *Handler) handleDislocCheck(w http.ResponseWriter, r *http.Request) {
	go h.dislocCheck.Run()
	h.writeJSON(w, http.StatusAccepted, map[string]any{"status": "success", "message": "disloc check started"})
}

func (h *Handler) proxyJSONPost(w http.ResponseWriter, r *http.Request, suffix string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(h.cfg.BaseSOAPServer, "/")+suffix, bytes.NewReader(body))
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		h.writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	writeProxyResponse(w, resp.StatusCode, responseBody)
}
