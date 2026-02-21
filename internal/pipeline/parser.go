package pipeline

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/jszwec/csvutil"
	"github.com/kurochkinivan/device_reporter/internal/domain"
)

type Parser struct {
	log          *slog.Logger
	files        <-chan string
	parseResults chan<- *domain.ParseResult
}

func NewParser(log *slog.Logger, files <-chan string, parseResults chan<- *domain.ParseResult) *Parser {
	return &Parser{
		log:          log,
		files:        files,
		parseResults: parseResults,
	}
}

func (p *Parser) Run(ctx context.Context) error {
	defer close(p.parseResults)

	for {
		select {
		case filename, ok := <-p.files:
			if !ok {
				return nil
			}

			p.log.DebugContext(ctx, "received file to parse", slog.String("filename", filename))

			devices, err := p.parseRecordsFromFile(filename)
			if err != nil {
				p.log.ErrorContext(ctx, "failed to parse records", slog.String("err", err.Error()))
			}

			p.parseResults <- &domain.ParseResult{
				Filename: filename,
				Devices:  devices,
				Error:    err,
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *Parser) parseRecordsFromFile(filename string) (_ []*domain.Device, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()

	return p.parseRecords(f)
}

func (p *Parser) parseRecords(r io.Reader) ([]*domain.Device, error) {
	reader := csv.NewReader(r)
	reader.Comma = '\t'

	dec, err := csvutil.NewDecoder(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	p.log.Debug("parsing records")

	var devices []*domain.Device
	for {
		var device domain.Device

		err := dec.Decode(&device)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return devices, fmt.Errorf("failed to decode device record: %w", err)
		}

		if err := device.Validate(); err != nil {
			return nil, fmt.Errorf("invalid device record #%d: %w", len(devices)+1, err)
		}

		devices = append(devices, &device)
	}

	p.log.Debug("successfully parsed records", slog.Int("device_count", len(devices)))

	return devices, nil
}
