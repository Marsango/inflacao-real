package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"inflacao-real/core/internal/database"
	"inflacao-real/core/internal/ingestion"
	"inflacao-real/core/internal/monitoring"
	"inflacao-real/core/internal/webapi"
)

func main() {
	if err := run(); err != nil {
		slog.Error("core startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := loadEnvironment(); err != nil {
		return err
	}
	requestContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startupContext, cancel := context.WithTimeout(requestContext, time.Minute)
	defer cancel()
	pool, err := openDatabase(startupContext)
	if err != nil {
		return err
	}
	defer pool.Close()
	application, err := newApplication(pool)
	if err != nil {
		return err
	}
	return application.run(requestContext)
}

func loadEnvironment() error {
	environmentFile := flag.String("env-file", "", "optional dotenv file; existing environment variables take precedence")
	flag.Parse()
	if *environmentFile == "" {
		return nil
	}
	if err := godotenv.Load(*environmentFile); err != nil {
		return fmt.Errorf("load environment file: %w", err)
	}
	return nil
}

func openDatabase(requestContext context.Context) (*sql.DB, error) {
	connectionString, err := database.ConnectionString(os.Getenv)
	if err != nil {
		return nil, err
	}
	pool, err := database.Open(requestContext, connectionString)
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(requestContext, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

type application struct {
	server *httpServer
	worker *ingestion.Worker
}

func newApplication(pool *sql.DB) (application, error) {
	auth := ingestion.AuthFromEnv(os.Getenv)
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: pool}, auth)
	if err != nil {
		return application{}, err
	}
	monitoring.RegisterRoutes(router, monitoring.PostgresStore{DB: pool}, auth.Authenticate)
	webapi.RegisterRoutes(router, webapi.PostgresStore{DB: pool})
	worker, err := createWorker(pool)
	if err != nil {
		return application{}, err
	}
	return application{server: newHTTPServer(router), worker: worker}, nil
}

func createWorker(pool *sql.DB) (*ingestion.Worker, error) {
	configuration, err := ingestion.WorkerConfigFromEnv(os.Getenv)
	if err != nil {
		return nil, err
	}
	return ingestion.NewWorker(pool, configuration)
}

func (application application) run(requestContext context.Context) error {
	stop := startWorker(requestContext, application.worker)
	defer stop()
	return application.server.serve(requestContext)
}

func startWorker(parentContext context.Context, worker *ingestion.Worker) func() {
	requestContext, cancel := context.WithCancel(parentContext)
	done := make(chan struct{})
	go runWorker(requestContext, worker, done)
	return func() { cancel(); <-done }
}

func runWorker(requestContext context.Context, worker *ingestion.Worker, done chan struct{}) {
	defer close(done)
	worker.Run(requestContext)
}
