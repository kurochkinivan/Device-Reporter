package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

type loggerKey struct{}

func main() {
	ctx := context.Background()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx = context.WithValue(ctx, loggerKey{}, log)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := cmd().Run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "stopped app due to the error %q\n", err)
		os.Exit(1)
	}
}
