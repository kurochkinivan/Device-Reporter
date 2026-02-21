package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/kurochkinivan/device_reporter/internal/app"
	"github.com/kurochkinivan/device_reporter/internal/config"
	altsrc "github.com/urfave/cli-altsrc/v3"
	"github.com/urfave/cli-altsrc/v3/yaml"
	"github.com/urfave/cli/v3"
)

var version = "dev"

func cmd() *cli.Command {
	return &cli.Command{
		Name:    "device_reporter",
		Usage:   "TSV pipeline service",
		Version: version,
		Flags:   flags(),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			log, ok := ctx.Value(loggerKey{}).(*slog.Logger)
			if !ok {
				return errors.New("failed to get logger from context")
			}

			cfg := config.Load(cmd)

			return app.New(log, cfg).Run(ctx)
		},
	}
}

func flags() []cli.Flag {
	var config string

	return []cli.Flag{
		&cli.StringFlag{
			Name:        "config",
			Aliases:     []string{"c"},
			Validator:   validateConfig,
			Usage:       "Load configuration from `FILE`",
			Destination: &config,
		},
		&cli.StringFlag{
			Name:      "watch-dir",
			Aliases:   []string{"w"},
			Usage:     "Set directory to watch for new files",
			Value:     "input",
			Sources:   cli.NewValueSourceChain(yaml.YAML("app.watch_dir", altsrc.NewStringPtrSourcer(&config))),
			Required:  true,
			Validator: validateDirectory,
		},
		&cli.StringFlag{
			Name:      "reports-dir",
			Aliases:   []string{"r"},
			Usage:     "Set directory to write reports to",
			Value:     "output",
			Sources:   cli.NewValueSourceChain(yaml.YAML("app.reports_dir", altsrc.NewStringPtrSourcer(&config))),
			Required:  true,
			Validator: validateDirectory,
		},
		&cli.DurationFlag{
			Name:     "scan-interval",
			Aliases:  []string{"s"},
			Value:    3 * time.Second,
			Usage:    "Set directory scan interval",
			Sources:  cli.NewValueSourceChain(yaml.YAML("app.scan_interval", altsrc.NewStringPtrSourcer(&config))),
			Required: true,
		},
		&cli.StringFlag{
			Name:     "pg-host",
			Usage:    "Set PostgreSQL host",
			Value:    "localhost",
			Sources:  cli.NewValueSourceChain(yaml.YAML("postgresql.host", altsrc.NewStringPtrSourcer(&config))),
			Required: true,
		},
		&cli.StringFlag{
			Name:     "pg-port",
			Usage:    "Set PostgreSQL port",
			Value:    "5432",
			Sources:  cli.NewValueSourceChain(yaml.YAML("postgresql.port", altsrc.NewStringPtrSourcer(&config))),
			Required: true,
		},
		&cli.StringFlag{
			Name:     "pg-username",
			Usage:    "Set PostgreSQL username",
			Sources:  cli.NewValueSourceChain(yaml.YAML("postgresql.username", altsrc.NewStringPtrSourcer(&config))),
			Required: true,
		},
		&cli.StringFlag{
			Name:     "pg-password",
			Usage:    "Set PostgreSQL password",
			Sources:  cli.NewValueSourceChain(yaml.YAML("postgresql.password", altsrc.NewStringPtrSourcer(&config))),
			Required: true,
		},
		&cli.StringFlag{
			Name:     "pg-dbname",
			Usage:    "Set PostgreSQL database name",
			Value:    "device_reporter",
			Sources:  cli.NewValueSourceChain(yaml.YAML("postgresql.dbname", altsrc.NewStringPtrSourcer(&config))),
			Required: true,
		},
		&cli.StringFlag{
			Name:    "http-host",
			Usage:   "Set HTTP server host",
			Value:   "localhost",
			Sources: cli.NewValueSourceChain(yaml.YAML("http.host", altsrc.NewStringPtrSourcer(&config))),
		},
		&cli.StringFlag{
			Name:    "http-port",
			Usage:   "Set HTTP server port",
			Value:   "8080",
			Sources: cli.NewValueSourceChain(yaml.YAML("http.port", altsrc.NewStringPtrSourcer(&config))),
		},
		&cli.DurationFlag{
			Name:    "http-idle-timeout",
			Usage:   "Set HTTP server idle timeout",
			Value:   1 * time.Minute,
			Sources: cli.NewValueSourceChain(yaml.YAML("http.idle_timeout", altsrc.NewStringPtrSourcer(&config))),
		},
		&cli.DurationFlag{
			Name:    "http-read-timeout",
			Usage:   "Set HTTP server read timeout",
			Value:   15 * time.Second,
			Sources: cli.NewValueSourceChain(yaml.YAML("http.read_timeout", altsrc.NewStringPtrSourcer(&config))),
		},
		&cli.DurationFlag{
			Name:    "http-write-timeout",
			Usage:   "Set HTTP server write timeout",
			Value:   15 * time.Second,
			Sources: cli.NewValueSourceChain(yaml.YAML("http.write_timeout", altsrc.NewStringPtrSourcer(&config))),
		},
	}
}

func validateDirectory(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%q does not exist", dir)
		}
		return fmt.Errorf("failed to stat %q: %w", dir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}

	return nil
}

func validateConfig(config string) error {
	info, err := os.Stat(config)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%q does not exist", config)
		}
		return fmt.Errorf("failed to stat %q: %w", config, err)
	}

	if info.IsDir() {
		return fmt.Errorf("%q is a directory, not a file", config)
	}

	ext := filepath.Ext(info.Name())
	if ext != ".yml" && ext != ".yaml" {
		return fmt.Errorf("invalid extension %q", config)
	}

	return nil
}
