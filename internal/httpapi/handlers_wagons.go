package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) handleWagonSearch(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		WagonNumber string `json:"wag_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if !isEightDigits(payload.WagonNumber) {
		h.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "wag_number must contain 8 digits"})
		return
	}
	h.proxyJSONPost(w, r, "/wagon/search", map[string]string{"wag_number": payload.WagonNumber})
}

func (h *Handler) handleWagonSearchList(w http.ResponseWriter, r *http.Request) {
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	wagons := normalizeWagonList(raw["wagons"])
	if len(wagons) == 0 {
		wagons = normalizeWagonList(raw["wagon_number"])
	}
	if len(wagons) == 0 {
		h.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "wagons list is required"})
		return
	}
	for _, wagon := range wagons {
		if !isEightDigits(wagon) {
			h.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "all wagon numbers must contain 8 digits"})
			return
		}
	}
	h.proxyJSONPost(w, r, "/wagon/search/list", map[string]any{"wagons": wagons})
}

func (h *Handler) handleAPIWagons(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT DISTINCT ON (wagon_number) id, wagon_number, source, invoice_number, pps_number, filed_at, cleaned_at, opened_payload, closed_payload, created_at, updated_at
		FROM wagon_visits
		ORDER BY wagon_number, COALESCE(cleaned_at, filed_at, created_at) DESC, id DESC`)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer rows.Close()
	h.writeJSON(w, http.StatusOK, readVisits(rows))
}

func (h *Handler) handleAPIWagonsFiled(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT id, wagon_number, source, invoice_number, pps_number, filed_at, cleaned_at, opened_payload, closed_payload, created_at, updated_at
		FROM wagon_visits
		WHERE cleaned_at IS NULL
		ORDER BY COALESCE(filed_at, created_at) DESC, id DESC`)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer rows.Close()
	h.writeJSON(w, http.StatusOK, readVisits(rows))
}

func (h *Handler) handleAPIWagonsClean(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT id, wagon_number, source, invoice_number, pps_number, filed_at, cleaned_at, opened_payload, closed_payload, created_at, updated_at
		FROM wagon_visits
		WHERE cleaned_at IS NOT NULL
		ORDER BY COALESCE(cleaned_at, updated_at) DESC, id DESC`)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer rows.Close()
	h.writeJSON(w, http.StatusOK, readVisits(rows))
}

func (h *Handler) handleAPIWagonHistory(w http.ResponseWriter, r *http.Request) {
	wagon := chi.URLParam(r, "wagon_number")
	rows, err := h.db.Query(r.Context(), `
		SELECT id, wagon_number, source, invoice_number, pps_number, filed_at, cleaned_at, opened_payload, closed_payload, created_at, updated_at
		FROM wagon_visits
		WHERE wagon_number = $1
		ORDER BY COALESCE(filed_at, created_at) DESC, id DESC`, wagon)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer rows.Close()
	h.writeJSON(w, http.StatusOK, readVisits(rows))
}

type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
}

func readVisits(rows pgxRows) []wagonVisit {
	var visits []wagonVisit
	for rows.Next() {
		var visit wagonVisit
		var invoice, pps sql.NullString
		var filedAt, cleanedAt sql.NullTime
		if err := rows.Scan(
			&visit.ID, &visit.WagonNumber, &visit.Source, &invoice, &pps, &filedAt, &cleanedAt,
			&visit.OpenedPayload, &visit.ClosedPayload, &visit.CreatedAt, &visit.UpdatedAt,
		); err != nil {
			continue
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
	return visits
}
