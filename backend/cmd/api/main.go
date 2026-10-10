package main

import (
	"context"
	"log"

	"github.com/labstack/echo/v4"

	"github.com/catechlounge/kyoya67-todo/backend/internal/config"
	"github.com/catechlounge/kyoya67-todo/backend/internal/database"
	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
	"github.com/catechlounge/kyoya67-todo/backend/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	queries := sqlc.New(pool)

	e := echo.New()

	router.Register(e)

	_ = queries

	e.Logger.Fatal(e.Start(":8080"))
}
