package main

import (
	"context"
	"crypto/tls"
	"log"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"github.com/catechlounge/kyoya67-todo/backend/internal/auth"
	"github.com/catechlounge/kyoya67-todo/backend/internal/config"
	"github.com/catechlounge/kyoya67-todo/backend/internal/database"
	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
	"github.com/catechlounge/kyoya67-todo/backend/internal/middleware"
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
	e.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	var authService *auth.Service
	authHandler := auth.NewHandler(nil, "")
	if cfg.WebAuthnRPID != "" {
		redisOptions := &redis.Options{Addr: cfg.RedisAddress}
		if cfg.RedisTLS {
			redisOptions.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		redisClient := redis.NewClient(redisOptions)
		defer redisClient.Close()
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := redisClient.Ping(pingCtx).Err()
		cancel()
		if err != nil {
			log.Fatalf("connect Redis: %v", err)
		}
		verifier, err := auth.NewWebAuthn(cfg.WebAuthnRPID, cfg.WebAuthnOrigin)
		if err != nil {
			log.Fatalf("configure WebAuthn: %v", err)
		}
		authService = auth.NewService(verifier, auth.NewRepository(pool), auth.NewChallengeStore(redisClient))
		authHandler = auth.NewHandler(authService, cfg.WebAuthnOrigin)
	}
	router.Register(e, queries, authHandler)

	e.Logger.Fatal(e.Start(":" + cfg.AppPort))
}
