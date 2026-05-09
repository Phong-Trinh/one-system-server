package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"one-system-server/internal/app"
)

func main() {
	// Pretty console logging for development
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05"})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	application, err := app.New(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize application")
	}
	defer application.Close()

	if err := application.Run(); err != nil {
		log.Fatal().Err(err).Msg("server stopped")
	}
}
