package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dislocservice/internal/bootstrap"
	"dislocservice/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	application, err := bootstrap.New(cfg)
	if err != nil {
		log.Fatalf("init app: %v", err)
	}
	defer application.Close()

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           application.Handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		application.Logger.Info("server starting on :" + cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			application.Logger.Error("server failed: " + err.Error())
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	application.Logger.Info("shutting down server")
	if err := server.Shutdown(ctx); err != nil {
		application.Logger.Error("shutdown failed: " + err.Error())
	}
}
