package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"dislocservice/internal/config"
	"dislocservice/internal/disloccheck"
	"dislocservice/internal/httpapi"
	"dislocservice/internal/logger"
	"dislocservice/internal/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Application struct {
	Config  config.Config
	DB      *pgxpool.Pool
	Logger  *logger.Logger
	Handler http.Handler

	logFile *os.File
}

func New(cfg config.Config) (*Application, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := postgres.NewPool(ctx, cfg.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}

	logPath := filepath.Join(cfg.LogDir, "service.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("open log file: %w", err)
	}

	appLogger := logger.New(logFile)
	dislocCheckService := disloccheck.NewService(cfg, appLogger)
	if err := postgres.InitSchema(ctx, db); err != nil {
		db.Close()
		_ = logFile.Close()
		return nil, err
	}

	handler := httpapi.NewHandler(cfg, db, &http.Client{Timeout: 5 * time.Minute}, appLogger, dislocCheckService)
	return &Application{
		Config:  cfg,
		DB:      db,
		Logger:  appLogger,
		Handler: httpapi.NewRouter(handler),
		logFile: logFile,
	}, nil
}

func (a *Application) Close() {
	if a.DB != nil {
		a.DB.Close()
	}
	if a.logFile != nil {
		_ = a.logFile.Close()
	}
}
