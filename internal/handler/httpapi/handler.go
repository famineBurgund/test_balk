package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	pgrepo "dislocservice/internal/repository/postgres"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Service interface {
	LastInvoiceInterval(ctx context.Context, direction string) (*time.Time, error)
	SaveInvoiceBatch(ctx context.Context, direction string, payload []byte) error
	ProxyJSONPost(ctx context.Context, suffix string, payload any) (int, []byte, error)
	FetchDisloc(ctx context.Context, endpoint string) (int, []byte, error)
	ListLatestWagons(ctx context.Context) ([]pgrepo.WagonVisit, error)
	ListFiledWagons(ctx context.Context) ([]pgrepo.WagonVisit, error)
	ListCleanWagons(ctx context.Context) ([]pgrepo.WagonVisit, error)
	WagonHistory(ctx context.Context, wagon string) ([]pgrepo.WagonVisit, error)
	RecentLogs(minutes int) ([]string, error)
}

type DislocCheckRunner interface {
	Run()
}

type Logger interface {
	Info(message string)
}

type Handler struct {
	service     Service
	logger      Logger
	dislocCheck DislocCheckRunner
}

func NewHandler(service Service, appLogger Logger, dislocCheck DislocCheckRunner) *Handler {
	return &Handler{
		service:     service,
		logger:      appLogger,
		dislocCheck: dislocCheck,
	}
}

func (h *Handler) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.logger.Info(fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		next.ServeHTTP(w, r)
	})
}

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

func (h *Handler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeProxyResponse(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func normalizeWagonList(value any) []string {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{strings.TrimSpace(v)}
	case []any:
		var out []string
		for _, item := range v {
			if str, ok := item.(string); ok && strings.TrimSpace(str) != "" {
				out = append(out, strings.TrimSpace(str))
			}
		}
		return out
	default:
		return nil
	}
}

func isTenDigits(value string) bool {
	return len(value) == 10 && strings.Trim(value, "0123456789") == ""
}

func isEightDigits(value string) bool {
	return len(value) == 8 && strings.Trim(value, "0123456789") == ""
}
