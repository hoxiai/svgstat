package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/hoxiai/svgstat/internal/config"
	"github.com/hoxiai/svgstat/internal/database"
	"github.com/hoxiai/svgstat/internal/migrate"
)

func main() {
	_ = godotenv.Load()
	setupLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info().Msg("Received shutdown signal")
		cancel()
	}()

	cfg := config.Load()

	db, err := database.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	migrator := migrate.New(db.Pool)

	var cmd string
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "up":
		log.Info().Msg("Running migrations...")
		if err := migrator.Up(ctx); err != nil {
			log.Fatal().Err(err).Msg("Migration failed")
		}
	case "status":
		log.Info().Msg("Checking migration status...")
		if err := migrator.Status(ctx); err != nil {
			log.Fatal().Err(err).Msg("Failed to check status")
		}
	default:
		fmt.Println("Usage:")
		fmt.Println("  migrate up      - Apply pending migrations")
		fmt.Println("  migrate status  - Show migration status")
		os.Exit(1)
	}
}

func setupLogger() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	})
}
