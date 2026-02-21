package pipeline_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/kurochkinivan/device_reporter/internal/domain"
	"github.com/kurochkinivan/device_reporter/internal/pipeline"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWriter_Run_ErrorIsNil(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	device := &domain.Device{
		N:         1,
		MQTT:      "",
		InvID:     "G-044322",
		UnitGUID:  "01749246-95f6-57db-b7c3-2ae0e8be671f",
		MsgID:     "cold7_Defrost_status",
		Text:      "Разморозка",
		Context:   "",
		Class:     "waiting",
		Level:     100,
		Area:      "LOCAL",
		Addr:      "cold7_status.Defrost_status",
		Block:     "",
		Type:      "",
		Bit:       "",
		InvertBit: "",
	}

	parseResult := &domain.ParseResult{
		Filename: "test.tsv",
		Error:    nil,
		Devices:  []*domain.Device{device},
	}

	parseResults := make(chan *domain.ParseResult, 1)
	reports := make(chan *domain.ParseResult, 1)

	mockTransactor := NewMockTransactor(t)
	mockDevicesSaver := NewMockDevicesSaver(t)
	mockFileUpdater := NewMockFileUpdater(t)

	mockTransactor.EXPECT().WithTransaction(mock.Anything, mock.Anything).
		Return(nil).
		Run(func(ctx context.Context, fn func(ctx context.Context) error) {
			_ = fn(ctx)
		})

	mockDevicesSaver.EXPECT().SaveDevices(mock.Anything, mock.Anything).Return(nil)
	mockFileUpdater.EXPECT().UpdateOrCreateFile(mock.Anything, mock.Anything).Return(nil)

	writer := pipeline.NewWriter(log, parseResults, reports, mockFileUpdater, mockDevicesSaver, mockTransactor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- writer.Run(ctx)
	}()

	go func() {
		parseResults <- parseResult
	}()

	// Expect result to be sent to reports channel
	select {
	case result := <-reports:
		if result == nil {
			t.Fatal("expected result, got nil")
		}
	case err := <-errChan:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: report was not sent to channel")
	}

	cancel()
	close(parseResults)

	select {
	case err := <-errChan:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: error was not sent to channel")
	}
}

func TestWriter_Run_ErrorIsNotNil(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	parseError := errors.New("parse error")
	parseResult := &domain.ParseResult{
		Filename: "test.tsv",
		Error:    parseError,
		Devices:  nil,
	}

	parseResults := make(chan *domain.ParseResult, 1)
	reports := make(chan *domain.ParseResult, 1)

	mockTransactor := NewMockTransactor(t)
	mockDevicesSaver := NewMockDevicesSaver(t)
	mockFileUpdater := NewMockFileUpdater(t)

	mockFileUpdater.EXPECT().UpdateOrCreateFile(mock.Anything, mock.Anything).Return(nil)

	writer := pipeline.NewWriter(log, parseResults, reports, mockFileUpdater, mockDevicesSaver, mockTransactor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- writer.Run(ctx)
	}()

	go func() {
		parseResults <- parseResult
	}()

	// Expect result to be sent to reports channel even when error is not nil
	select {
	case result := <-reports:
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		if result.Error == nil {
			t.Fatal("expected error in result, got nil")
		}
	case err := <-errChan:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: report was not sent to channel")
	}

	cancel()
	close(parseResults)

	select {
	case err := <-errChan:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: error was not sent to channel")
	}
}

func TestWriter_Run_ChannelCloses(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	parseResults := make(chan *domain.ParseResult, 1)
	reports := make(chan *domain.ParseResult, 1)

	mockTransactor := NewMockTransactor(t)
	mockDevicesSaver := NewMockDevicesSaver(t)
	mockFileUpdater := NewMockFileUpdater(t)

	writer := pipeline.NewWriter(log, parseResults, reports, mockFileUpdater, mockDevicesSaver, mockTransactor)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- writer.Run(ctx)
	}()

	close(parseResults)

	select {
	case err := <-errChan:
		require.NoError(t, err)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: error was not sent to channel")
	}
}
