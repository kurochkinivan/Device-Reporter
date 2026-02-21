package pipeline_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/kurochkinivan/device_reporter/internal/domain"
	"github.com/kurochkinivan/device_reporter/internal/pipeline"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReporter_Run_HappyPath(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	device := &domain.Device{
		N:         1,
		MQTT:      "test/mqtt",
		InvID:     "G-044322",
		UnitGUID:  "01749246-95f6-57db-b7c3-2ae0e8be671f",
		MsgID:     "cold7_Defrost_status",
		Text:      "Разморозка",
		Context:   "test",
		Class:     "waiting",
		Level:     100,
		Area:      "LOCAL",
		Addr:      "cold7_status.Defrost_status",
		Block:     "block1",
		Type:      "type1",
		Bit:       "bit1",
		InvertBit: "invbit1",
	}

	parseResult := &domain.ParseResult{
		Filename: "test.tsv",
		Error:    nil,
		Devices:  []*domain.Device{device},
	}

	reports := make(chan *domain.ParseResult, 1)

	mockReportGenerator := NewMockReportGenerator(t)
	mockReportGenerator.EXPECT().
		GenerateReport(mock.MatchedBy(func(path string) bool {
			return path != ""
		}), device.UnitGUID, parseResult.Filename, mock.MatchedBy(func(devices []*domain.Device) bool {
			return len(devices) == 1 && devices[0].UnitGUID == device.UnitGUID
		})).
		Return(nil)

	reporter := pipeline.NewReporter(log, "/tmp", reports, mockReportGenerator)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- reporter.Run(ctx)
	}()

	go func() {
		reports <- parseResult
	}()

	// Give reporter time to process
	select {
	case <-time.After(10 * time.Millisecond):
		// Expected - reporter should process the result
	case err := <-errChan:
		t.Fatalf("unexpected error: %v", err)
	}

	cancel()
	close(reports)

	select {
	case err := <-errChan:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: error was not sent to channel")
	}
}

func TestReporter_Run_EmptyDevices(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	parseResult := &domain.ParseResult{
		Filename: "empty.tsv",
		Error:    nil,
		Devices:  []*domain.Device{}, // Empty devices list
	}

	reports := make(chan *domain.ParseResult, 1)

	mockReportGenerator := NewMockReportGenerator(t)
	// GenerateReport should NOT be called when devices list is empty
	mockReportGenerator.AssertNotCalled(t, "GenerateReport")

	reporter := pipeline.NewReporter(log, "/tmp", reports, mockReportGenerator)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- reporter.Run(ctx)
	}()

	go func() {
		reports <- parseResult
	}()

	// Give reporter time to process
	select {
	case <-time.After(10 * time.Millisecond):
		// Expected - reporter should process the result without errors
	case err := <-errChan:
		t.Fatalf("unexpected error: %v", err)
	}

	cancel()
	close(reports)

	select {
	case err := <-errChan:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: error was not sent to channel")
	}
}

func TestReporter_Run_ChannelCloses(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	reports := make(chan *domain.ParseResult, 1)

	mockReportGenerator := NewMockReportGenerator(t)

	reporter := pipeline.NewReporter(log, "/tmp", reports, mockReportGenerator)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- reporter.Run(ctx)
	}()

	// Close the reports channel immediately
	close(reports)

	select {
	case err := <-errChan:
		require.NoError(t, err, "expected nil error when channel closes")
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: error was not sent to channel")
	}
}
