package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"inflacao-real/core/internal/database"
	"inflacao-real/core/internal/ingestion"
)

func main() {
	if err := run(); err != nil {
		slog.Error("core startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	envFile := flag.String("env-file", "", "optional dotenv file; existing environment variables take precedence")
	flag.Parse()
	if *envFile != "" {
		if err := godotenv.Load(*envFile); err != nil {
			return fmt.Errorf("load environment file: %w", err)
		}
	}

	connectionString, err := database.ConnectionString(os.Getenv)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startupCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	db, err := database.Open(startupCtx, connectionString)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := database.Migrate(startupCtx, db); err != nil {
		return err
	}

	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: db}, ingestion.AuthFromEnv(os.Getenv))
	if err != nil {
		return err
	}
	workerConfig, err := ingestion.WorkerConfigFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	worker, err := ingestion.NewWorker(db, workerConfig)
	if err != nil {
		return err
	}
	workerCtx, cancelWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		worker.Run(workerCtx)
	}()
	defer func() {
		cancelWorker()
		<-workerDone
	}()
	address := os.Getenv("HTTP_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	server := &http.Server{
		Addr:              address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       time.Minute,
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	slog.Info("ingestion API started", "address", address)
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			server.Close()
			return fmt.Errorf("shut down HTTP: %w", err)
		}
		return nil
	}
}
