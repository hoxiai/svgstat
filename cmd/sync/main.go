package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/svgstat/svgstat/internal/analytics"
	"github.com/svgstat/svgstat/internal/cache"
	"github.com/svgstat/svgstat/internal/config"
	"github.com/svgstat/svgstat/internal/database"
	"github.com/svgstat/svgstat/internal/project"
	"github.com/svgstat/svgstat/internal/worker"
)

func main() {
	_ = godotenv.Load()
	setupLogger()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info().Msg("Received interrupt signal")
		cancel()
	}()

	cfg := config.Load()

	db, err := database.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	c, err := cache.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer c.Close()

	projectRepo := project.NewPostgresRepository(db.Pool)
	analyticsSvc := analytics.New(c, projectRepo, nil, cfg.Analytics.KeyTTL, "")
	w := worker.New(analyticsSvc, projectRepo, db.Pool, cfg.Worker.FlushInterval)

	log.Info().Msg("Starting manual synchronization from Redis to PostgreSQL...")
	if err := w.FlushOnce(ctx); err != nil {
		log.Fatal().Err(err).Msg("Synchronization failed")
	}

	log.Info().Msg("Synchronization completed successfully!")
}

func setupLogger() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	})
}
