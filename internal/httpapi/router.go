package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(handler *Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(5 * time.Minute))
	r.Use(handler.requestLogger)

	r.Get("/soap/invoice/last-interval-at", handler.handleLastInterval("at"))
	r.Post("/soap/invoice/process-invoices-at", handler.handleProcessInvoices("at"))
	r.Get("/soap/invoice/last-interval-nmtp", handler.handleLastInterval("nmtp"))
	r.Post("/soap/invoice/process-invoices-nmtp", handler.handleProcessInvoices("nmtp"))

	r.Post("/soap/invoice/search", handler.handleInvoiceSearch)
	r.Post("/soap/invoice/idsearch", handler.handleInvoiceIDSearch)
	r.Post("/soap/wagon/search", handler.handleWagonSearch)
	r.Post("/soap/wagon/search/list", handler.handleWagonSearchList)
	r.Post("/soap/invoice/disloc-check", handler.handleDislocCheck)
	r.Post("/invoice/disloc-check", handler.handleDislocCheck)

	r.Get("/soap/disloc/attis", handler.handleDisloc("attis"))
	r.Get("/soap/disloc/nmtp", handler.handleDisloc("nmtp"))
	r.Get("/soap/disloc/ut_emd", handler.handleDisloc("ut_emd"))
	r.Get("/soap/disloc/gut_emd", handler.handleDisloc("gut_emd"))
	r.Get("/soap/disloc/at_emd", handler.handleDisloc("at_emd"))
	r.Get("/soap/disloc/filed-cars-at", handler.handleDisloc("filed-cars-at"))
	r.Get("/soap/disloc/filed-cars-nmtp", handler.handleDisloc("filed-cars-nmtp"))
	r.Get("/soap/disloc/pps-status-at", handler.handleDisloc("pps-status-at"))
	r.Get("/soap/disloc/pps-status-nmtp", handler.handleDisloc("pps-status-nmtp"))
	r.Get("/soap/logs", handler.handleLogs)

	r.Get("/api/wagons", handler.handleAPIWagons)
	r.Get("/api/wagons/filed", handler.handleAPIWagonsFiled)
	r.Get("/api/wagons/clean", handler.handleAPIWagonsClean)
	r.Get("/api/wagons/{wagon_number}", handler.handleAPIWagonHistory)

	return r
}
