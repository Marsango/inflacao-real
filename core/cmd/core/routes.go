package main

import (
	"context"
	"database/sql"
	"os"

	"github.com/gin-gonic/gin"

	"inflacao-real/core/internal/admin"
	"inflacao-real/core/internal/historycache"
	"inflacao-real/core/internal/ingestion"
	"inflacao-real/core/internal/monitoring"
	"inflacao-real/core/internal/webapi"
)

func newRouter(requestContext context.Context, pool *sql.DB) (*gin.Engine, error) {
	auth := ingestion.AuthFromEnv(os.Getenv)
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: pool}, auth)
	if err != nil {
		return nil, err
	}
	monitoring.RegisterRoutes(router, monitoring.PostgresStore{DB: pool}, auth.Authenticate)
	if err := registerWebAPI(requestContext, router, pool); err != nil {
		return nil, err
	}
	if err := admin.RegisterRoutes(router, pool, os.Getenv); err != nil {
		return nil, err
	}
	return router, nil
}

func registerWebAPI(requestContext context.Context, router *gin.Engine, pool *sql.DB) error {
	configuration, err := historycache.ConfigFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	store, err := historycache.New(requestContext, webapi.PostgresStore{DB: pool}, configuration)
	if err != nil {
		return err
	}
	webapi.RegisterRoutes(router, store)
	return nil
}
