package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) handleLastInterval(direction string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		latest, err := h.service.LastInvoiceInterval(ctx, direction)
		if err != nil {
			h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
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
		if err := h.service.SaveInvoiceBatch(ctx, direction, body); err != nil {
			h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
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

	status, body, err := h.service.ProxyJSONPost(r.Context(), "/invoice/search", map[string]string{"invoice_number": payload.InvoiceNumber})
	if err != nil {
		h.writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeProxyResponse(w, status, body)
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

	status, body, err := h.service.ProxyJSONPost(r.Context(), "/invoice/idsearch", map[string]string{"invoice_id": payload.InvoiceID})
	if err != nil {
		h.writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeProxyResponse(w, status, body)
}

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

	status, body, err := h.service.ProxyJSONPost(r.Context(), "/wagon/search", map[string]string{"wag_number": payload.WagonNumber})
	if err != nil {
		h.writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeProxyResponse(w, status, body)
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

	status, body, err := h.service.ProxyJSONPost(r.Context(), "/wagon/search/list", map[string]any{"wagons": wagons})
	if err != nil {
		h.writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeProxyResponse(w, status, body)
}

func (h *Handler) handleDislocCheck(w http.ResponseWriter, r *http.Request) {
	go h.dislocCheck.Run()
	h.writeJSON(w, http.StatusAccepted, map[string]any{"status": "success", "message": "disloc check started"})
}

func (h *Handler) handleDisloc(endpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, body, err := h.service.FetchDisloc(r.Context(), endpoint)
		if err != nil {
			h.writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		writeProxyResponse(w, status, body)
	}
}

func (h *Handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	minutes := 5
	if raw := r.URL.Query().Get("minutes"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			minutes = value
		}
	}

	logs, err := h.service.RecentLogs(minutes)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"status": "success", "logs": logs})
}

func (h *Handler) handleAPIWagons(w http.ResponseWriter, r *http.Request) {
	visits, err := h.service.ListLatestWagons(r.Context())
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, visits)
}

func (h *Handler) handleAPIWagonsFiled(w http.ResponseWriter, r *http.Request) {
	visits, err := h.service.ListFiledWagons(r.Context())
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, visits)
}

func (h *Handler) handleAPIWagonsClean(w http.ResponseWriter, r *http.Request) {
	visits, err := h.service.ListCleanWagons(r.Context())
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, visits)
}

func (h *Handler) handleAPIWagonHistory(w http.ResponseWriter, r *http.Request) {
	visits, err := h.service.WagonHistory(r.Context(), chi.URLParam(r, "wagon_number"))
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, visits)
}
