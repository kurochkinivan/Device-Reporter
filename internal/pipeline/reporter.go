package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/kurochkinivan/device_reporter/internal/domain"
)

type Reporter struct {
	log             *slog.Logger
	outputDir       string
	reports         <-chan *domain.ParseResult
	reportGenerator ReportGenerator
}

func NewReporter(
	log *slog.Logger,
	outputDir string,
	reports <-chan *domain.ParseResult,
	reportGenerator ReportGenerator,
) *Reporter {
	return &Reporter{
		log:             log,
		outputDir:       outputDir,
		reports:         reports,
		reportGenerator: reportGenerator,
	}
}

func (r *Reporter) Run(ctx context.Context) error {
	for {
		select {
		case result, ok := <-r.reports:
			if !ok {
				return nil
			}

			log := r.log.With(
				slog.String("filename", result.Filename),
				slog.Int("devices_count", len(result.Devices)),
			)

			log.InfoContext(ctx, "received parse result, generating report")

			if err := r.processResult(result); err != nil {
				log.InfoContext(ctx, "failed to generate report", slog.String("err", err.Error()))
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *Reporter) processResult(result *domain.ParseResult) error {
	// группируем устройства по unit_guid
	byGUID := make(map[string][]*domain.Device)
	for _, d := range result.Devices {
		byGUID[d.UnitGUID] = append(byGUID[d.UnitGUID], d)
	}

	// для каждого guid генерируем отдельный PDF
	for guid, devices := range byGUID {
		path := filepath.Join(r.outputDir, guid+".pdf")

		if err := r.reportGenerator.GenerateReport(path, guid, result.Filename, devices); err != nil {
			return fmt.Errorf("guid %s: %w", guid, err)
		}
	}

	return nil
}
