package postgresql

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kurochkinivan/device_reporter/internal/config"
)

const (
	maxRetries = 5
	retryDelay = 5 * time.Second
)

func NewConnection(ctx context.Context, log *slog.Logger, cfg config.PostgreSQL) (*pgxpool.Pool, error) {
	connectionURL := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.Username, cfg.Password),
		Host:     net.JoinHostPort(cfg.Host, cfg.Port),
		Path:     cfg.DBName,
		RawQuery: "sslmode=disable",
	}

	pool, err := pgxpool.New(ctx, connectionURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	retry := Retry(log, pool.Ping, maxRetries, retryDelay)

	if err := retry(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping pool: %w", err)
	}

	return pool, nil
}

type PingFunction func(context.Context) error

func Retry(log *slog.Logger, ping PingFunction, retries int, delay time.Duration) PingFunction {
	return func(ctx context.Context) error {
		for r := 0; ; r++ {
			err := ping(ctx)
			if err == nil || r >= retries {
				return err
			}

			log.Debug("database connection attempt failed, retrying",
				slog.Int("attempt", r+1),
				slog.Int("max_retries", retries),
				slog.String("err", err.Error()))

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
