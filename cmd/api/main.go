package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/svgstat/svgstat/internal/api"
	"github.com/svgstat/svgstat/internal/config"
)

func main() {
	_ = godotenv.Load()

	setupLogger()

	log.Info().Msg("Starting SVGStat...")

	cfg := config.Load()

	app, err := api.NewApp(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create app")
	}
	defer app.Close()

	router := app.SetupRoutes()

	workerCtx, stopWorker := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		app.StartWorker(workerCtx)
		close(workerDone)
	}()

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	go func() {
		log.Info().Str("addr", cfg.HTTP.Addr).Msg("Server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	// Stop the worker and wait for its final flush before the deferred
	// app.Close() tears down the database pool and Redis client.
	stopWorker()
	<-workerDone

	log.Info().Msg("Server exited")
}

func setupLogger() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	})
}
