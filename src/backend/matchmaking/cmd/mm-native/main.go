package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"voice/backend/matchmaking/internal/nativeapp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := nativeapp.Run(ctx, logger); err != nil {
		logger.Error("native matchmaking stopped without qualification")
		os.Exit(1)
	}
}
