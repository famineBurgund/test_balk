package httpapi

import (
	"fmt"
	"net/http"

	"dislocservice/internal/config"
	"dislocservice/internal/logger"
)

type Handler struct {
	cfg         config.Config
	db          DB
	client      HTTPClient
	logger      *logger.Logger
	dislocCheck DislocCheckRunner
}

func NewHandler(cfg config.Config, db DB, client HTTPClient, appLogger *logger.Logger, dislocCheck DislocCheckRunner) *Handler {
	return &Handler{
		cfg:         cfg,
		db:          db,
		client:      client,
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
