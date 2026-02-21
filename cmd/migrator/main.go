package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const (
	migrationTypeUp   = "up"
	migrationTypeDown = "down"
)

const (
	exitCodeOK = iota
	exitCodeInputErr
	exitCodeInternalErr
)

type flags struct {
	migrationType string
	username      string
	password      string
	host          string
	port          string
	db            string
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	exitCode, err := Run(ctx, log)
	if err != nil {
		log.ErrorContext(ctx, "failed to apply migrations", slog.String("err", err.Error()))
	}

	stop()
	os.Exit(exitCode)
}

func Run(ctx context.Context, log *slog.Logger) (exitCode int, err error) {
	f := parseFlags()

	if err := f.validate(); err != nil {
		return exitCodeInputErr, fmt.Errorf("invalid flags: %w", err)
	}

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return exitCodeInternalErr, fmt.Errorf("failed to create migrations source: %w", err)
	}

	migrator, err := migrate.NewWithSourceInstance("iofs", src, f.databaseURL())
	if err != nil {
		return exitCodeInternalErr, fmt.Errorf("failed to create migrator: %w", err)
	}
	if err != nil {
		return exitCodeInternalErr, fmt.Errorf("failed to create migrator: %w", err)
	}
	defer func() {
		srcErr, dbErr := migrator.Close()
		if closeErr := errors.Join(srcErr, dbErr); closeErr != nil {
			if err == nil {
				exitCode = exitCodeInternalErr
			}
			err = errors.Join(err, closeErr)
		}
	}()

	if err := applyMigration(migrator, f.migrationType); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.InfoContext(ctx, "no migrations to apply")
			return exitCodeOK, nil
		}

		return exitCodeInternalErr, fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.InfoContext(ctx, "migrations applied successfully", slog.String("type", f.migrationType))

	return exitCodeOK, nil
}

func applyMigration(migrator *migrate.Migrate, migrationType string) error {
	switch migrationType {
	case migrationTypeUp:
		return migrator.Up()
	case migrationTypeDown:
		return migrator.Down()
	default:
		return fmt.Errorf("unknown migration type %q", migrationType)
	}
}

func parseFlags() *flags {
	f := &flags{}
	flag.StringVar(&f.migrationType, "type", migrationTypeUp, "migration type: up/down")
	flag.StringVar(&f.username, "username", "", "database username")
	flag.StringVar(&f.password, "password", "", "database password")
	flag.StringVar(&f.host, "host", "127.0.0.1", "database host")
	flag.StringVar(&f.port, "port", "5432", "database port")
	flag.StringVar(&f.db, "db", "", "database name")
	flag.Parse()
	return f
}

func (f *flags) validate() error {
	if f.migrationType != migrationTypeUp && f.migrationType != migrationTypeDown {
		return fmt.Errorf("type must be %q or %q, got %q", migrationTypeUp, migrationTypeDown, f.migrationType)
	}

	for _, req := range []struct{ name, value string }{
		{"username", f.username},
		{"password", f.password},
		{"db", f.db},
		{"port", f.port},
	} {
		if req.value == "" {
			return fmt.Errorf("%s is required", req.name)
		}
	}

	return nil
}

func (f *flags) databaseURL() string {
	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(f.username, f.password),
		Host:     net.JoinHostPort(f.host, f.port),
		Path:     f.db,
		RawQuery: "sslmode=disable",
	}).String()
}
