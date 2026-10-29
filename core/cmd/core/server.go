package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type httpServer struct{ server *http.Server }

func newHTTPServer(handler http.Handler) *httpServer {
	address := os.Getenv("HTTP_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: time.Minute}
	return &httpServer{server: server}
}

func (server *httpServer) serve(requestContext context.Context) error {
	failures := make(chan error, 1)
	go server.listen(failures)
	slog.Info("ingestion API started", "address", server.server.Addr)
	select {
	case err := <-failures:
		return serverFailure(err)
	case <-requestContext.Done():
		return server.shutdown()
	}
}

func (server *httpServer) listen(failures chan<- error) {
	failures <- server.server.ListenAndServe()
}

func serverFailure(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("serve HTTP: %w", err)
}

func (server *httpServer) shutdown() error {
	requestContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.server.Shutdown(requestContext); err != nil {
		server.server.Close()
		return fmt.Errorf("shut down HTTP: %w", err)
	}
	return nil
}
