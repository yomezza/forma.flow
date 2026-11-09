package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yomezza/forma.flow/backend/internal/config"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
	httptransport "github.com/yomezza/forma.flow/backend/internal/transport/http"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	setup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	db, err := postgres.Open(setup, cfg.DatabaseURL, cfg.PoolMax)
	if err != nil {
		return errors.New("database connection failed")
	}
	defer db.Pool.Close()
	if err = db.CheckSchema(setup); err != nil {
		return err
	}
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: httptransport.New(db, log), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	log.Info("server listening", "address", cfg.HTTPAddr)
	select {
	case err = <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	return server.Shutdown(shutdown)
}
