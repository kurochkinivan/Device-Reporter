package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/kurochkinivan/device_reporter/internal/domain"
)

type Writer struct {
	log          *slog.Logger
	parseResults <-chan *domain.ParseResult
	reports      chan<- *domain.ParseResult
	fileUpdater  FileUpdater
	devicesSaver DevicesSaver
	transactor   Transactor
}

func NewWriter(
	log *slog.Logger,
	parseResults <-chan *domain.ParseResult,
	reports chan<- *domain.ParseResult,
	fileUpdater FileUpdater,
	devicesSaver DevicesSaver,
	transactor Transactor,
) *Writer {
	return &Writer{
		log:          log,
		parseResults: parseResults,
		reports:      reports,
		fileUpdater:  fileUpdater,
		devicesSaver: devicesSaver,
		transactor:   transactor,
	}
}

func (w *Writer) Run(ctx context.Context) error {
	defer close(w.reports)

	for {
		select {
		case result, ok := <-w.parseResults:
			if !ok {
				return nil
			}

			log := w.log.With(
				slog.String("filename", result.Filename),
				slog.Int("devices_count", len(result.Devices)),
			)

			log.InfoContext(ctx, "received parse result")

			if err := w.processParseResult(ctx, log, result); err != nil {
				log.ErrorContext(ctx, "failed to process parse result", slog.String("err", err.Error()))
				continue
			}

			w.reports <- result

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (w *Writer) processParseResult(ctx context.Context, log *slog.Logger, result *domain.ParseResult) error {
	switch result.Error {
	case nil:
		log.DebugContext(ctx, "saving parse result to database")

		err := w.saveResult(ctx, result)
		if err != nil {
			return fmt.Errorf("failed to save result: %w", err)
		}

		log.DebugContext(ctx, "result saved successfully")

	default:
		log.DebugContext(ctx, "processing error parse result")

		now := time.Now()
		err := w.fileUpdater.UpdateOrCreateFile(ctx, &domain.File{
			Name:         filepath.Base(result.Filename),
			Status:       domain.StatusError,
			ErrorMessage: result.Error.Error(),
			ProcessedAt:  &now,
		})
		if err != nil {
			return fmt.Errorf("failed to save parse result: %w", err)
		}
	}

	return nil
}

func (w *Writer) saveResult(ctx context.Context, result *domain.ParseResult) error {
	return w.transactor.WithTransaction(ctx, func(ctx context.Context) error {
		err := w.devicesSaver.SaveDevices(ctx, result.Devices...)
		if err != nil {
			return fmt.Errorf("failed to save devices: %w", err)
		}

		now := time.Now()
		err = w.fileUpdater.UpdateOrCreateFile(ctx, &domain.File{
			Name:        filepath.Base(result.Filename),
			Status:      domain.StatusDone,
			ProcessedAt: &now,
		})
		if err != nil {
			return fmt.Errorf("failed to update file status: %w", err)
		}

		return nil
	})
}
