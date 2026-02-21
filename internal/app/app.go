package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/kurochkinivan/device_reporter/internal/config"
	v1 "github.com/kurochkinivan/device_reporter/internal/controller/http/v1"
	"github.com/kurochkinivan/device_reporter/internal/domain"
	"github.com/kurochkinivan/device_reporter/internal/infrastructure/report_generator"
	"github.com/kurochkinivan/device_reporter/internal/pipeline"
	"github.com/kurochkinivan/device_reporter/internal/repository/postgresql"
	"golang.org/x/sync/errgroup"
)

const (
	filesBuffer        = 100
	parseResultsBuffer = 50
	reportsBuffer      = 100
)

type App struct {
	log *slog.Logger
	cfg *config.Config
}

func New(log *slog.Logger, cfg *config.Config) *App {
	return &App{
		log: log,
		cfg: cfg,
	}
}

func (a *App) Run(ctx context.Context) error {
	a.log.InfoContext(ctx, "starting app",
		slog.String("watch_dir", a.cfg.App.WatchDirectory),
		slog.String("reports_dir", a.cfg.App.ReportsDirectory),
		slog.Duration("scan_interval", a.cfg.App.DirectoryScanInterval),
	)

	a.log.InfoContext(ctx, "establishing postgresql connection",
		slog.String("postgresql_host", a.cfg.PostgreSQL.Host),
		slog.String("postgresql_port", a.cfg.PostgreSQL.Port),
		slog.String("postgresql_dbname", a.cfg.PostgreSQL.DBName),
	)

	pool, err := postgresql.NewConnection(ctx, a.log, a.cfg.PostgreSQL)
	if err != nil {
		return fmt.Errorf("failed to create db connection: %w", err)
	}
	defer pool.Close()

	filesRepository := postgresql.NewFilesRepository(pool)
	devicesRepository := postgresql.NewDevicesRepository(pool)
	txManager := postgresql.NewTxManager(pool)

	if err := filesRepository.ResetProcessingFiles(ctx); err != nil {
		return fmt.Errorf("failed to reset processing files: %w", err)
	}

	return a.startPipeline(ctx, filesRepository, devicesRepository, txManager)
}

func (a *App) startPipeline(
	ctx context.Context,
	filesRepo *postgresql.FilesRepository,
	devicesRepo *postgresql.DevicesRepository,
	txManager *postgresql.TxManager,
) error {
	files := make(chan string, filesBuffer)
	parseResults := make(chan *domain.ParseResult, parseResultsBuffer)
	reports := make(chan *domain.ParseResult, reportsBuffer)

	scanner := pipeline.NewScanner(
		a.log,
		a.cfg.WatchDirectory,
		a.cfg.DirectoryScanInterval,
		files,
		filesRepo,
		filesRepo,
	)
	parser := pipeline.NewParser(a.log, files, parseResults)
	writer := pipeline.NewWriter(a.log, parseResults, reports, filesRepo, devicesRepo, txManager)
	reporter := pipeline.NewReporter(a.log, a.cfg.ReportsDirectory, reports, report_generator.New())
	server := v1.NewServer(a.cfg.HTTP, devicesRepo)

	erg, ctx := errgroup.WithContext(ctx)

	erg.Go(func() error {
		a.log.InfoContext(ctx, "scanner started")
		return scanner.Run(ctx)
	})

	erg.Go(func() error {
		a.log.InfoContext(ctx, "parser started")
		return parser.Run(ctx)
	})

	erg.Go(func() error {
		a.log.InfoContext(ctx, "writer started")
		return writer.Run(ctx)
	})

	erg.Go(func() error {
		a.log.InfoContext(ctx, "reporter started")
		return reporter.Run(ctx)
	})

	erg.Go(func() error {
		a.log.InfoContext(ctx, "starting http server",
			slog.String("addr", net.JoinHostPort(a.cfg.HTTP.Host, a.cfg.HTTP.Port)),
		)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server error: %w", err)
		}

		return nil
	})

	erg.Go(func() error {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		return server.Shutdown(shutdownCtx)
	})

	a.log.InfoContext(ctx, "all components started")

	if err := erg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		a.log.ErrorContext(ctx, "pipeline stopped with error", slog.String("err", err.Error()))

		return err
	}

	a.log.InfoContext(ctx, "pipeline stopped gracefully")

	return nil
}
