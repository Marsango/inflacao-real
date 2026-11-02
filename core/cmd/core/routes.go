package main

import (
	"context"
	"database/sql"
	"os"

	"github.com/gin-gonic/gin"

	"inflacao-real/core/internal/admin"
	"inflacao-real/core/internal/historycache"
	"inflacao-real/core/internal/historyprecompute"
	"inflacao-real/core/internal/ingestion"
	"inflacao-real/core/internal/monitoring"
	"inflacao-real/core/internal/webapi"
)

func newRouter(requestContext context.Context, pool *sql.DB, precomputed *historyprecompute.Store) (*gin.Engine, error) {
	auth := ingestion.AuthFromEnv(os.Getenv)
	router, err := ingestion.NewRouter(ingestion.PostgresStore{DB: pool}, auth)
	if err != nil {
		return nil, err
	}
	historyprecompute.RegisterRoutes(router, precomputed, auth.Authenticate, auth.ScraperID, auth.ShopIDs)
	monitoring.RegisterRoutes(router, monitoring.PostgresStore{DB: pool}, auth.Authenticate)
	if err := registerWebAPI(requestContext, router, precomputed); err != nil {
		return nil, err
	}
	if err := admin.RegisterRoutes(router, pool, os.Getenv); err != nil {
		return nil, err
	}
	return router, nil
}

func registerWebAPI(requestContext context.Context, router *gin.Engine, precomputed *historyprecompute.Store) error {
	configuration, err := historycache.ConfigFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	store, err := historycache.New(requestContext, precomputed, configuration)
	if err != nil {
		return err
	}
	webapi.RegisterRoutes(router, store)
	return nil
}
