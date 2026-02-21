package pipeline_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/kurochkinivan/device_reporter/internal/domain"
	"github.com/kurochkinivan/device_reporter/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Run_HappyPath(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	expected := &domain.Device{
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

	filename := createTSV(t, expected)
	files := make(chan string, 1)
	go func() {
		files <- filename
	}()

	parseResults := make(chan *domain.ParseResult, 1)

	parser := pipeline.NewParser(log, files, parseResults)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- parser.Run(ctx)
	}()

	select {
	case result := <-parseResults:
		require.NotNil(t, result)
		assert.Len(t, result.Devices, 1)
		assert.Equal(t, expected, result.Devices[0])
	case err := <-errChan:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: parse result was not sent to channel")
	}

	cancel()

	select {
	case err := <-errChan:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: error was not sent to channel")
	}
}

func TestParser_Run_InvalidData(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	filename := createInvalidTSV(t)
	files := make(chan string, 1)
	go func() {
		files <- filename
	}()

	parseResults := make(chan *domain.ParseResult, 1)

	parser := pipeline.NewParser(log, files, parseResults)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- parser.Run(ctx)
	}()

	select {
	case result := <-parseResults:
		require.NotNil(t, result)
		require.Error(t, result.Error)

	case err := <-errChan:
		t.Fatalf("unexpected error: %v", err)

	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: parse result was not sent to channel")
	}

	cancel()

	select {
	case err := <-errChan:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: error was not sent to channel")
	}
}

func TestParser_Run_EmptyFile(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	filename := createEmptyTSV(t)
	files := make(chan string, 1)
	go func() {
		files <- filename
	}()

	parseResults := make(chan *domain.ParseResult, 1)

	parser := pipeline.NewParser(log, files, parseResults)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- parser.Run(ctx)
	}()

	select {
	case result := <-parseResults:
		require.NotNil(t, result)
		require.NoError(t, result.Error)
		assert.Empty(t, result.Devices)
	case err := <-errChan:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: parse result was not sent to channel")
	}

	cancel()

	select {
	case err := <-errChan:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: error was not sent to channel")
	}
}

func createTSV(t *testing.T, devices ...*domain.Device) string {
	f, err := os.CreateTemp(t.TempDir(), "*.tsv")
	require.NoError(t, err)

	_, err = fmt.Fprintf(
		f,
		"n\tmqtt\tinvid\tunit_guid\tmsg_id\ttext\tcontext\tclass\tlevel\tarea\taddr\tblock\ttype\tbit\tinvert_bit\n",
	)
	require.NoError(t, err)

	for _, device := range devices {
		_, err = fmt.Fprintf(f, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			device.N, device.MQTT, device.InvID, device.UnitGUID,
			device.MsgID, device.Text, device.Context, device.Class,
			device.Level, device.Area, device.Addr, device.Block,
			device.Type, device.Bit, device.InvertBit,
		)
		require.NoError(t, err)
	}

	require.NoError(t, f.Close())

	return f.Name()
}

func createInvalidTSV(t *testing.T) string {
	f, err := os.CreateTemp(t.TempDir(), "*.tsv")
	require.NoError(t, err)

	_, err = fmt.Fprintf(
		f,
		"n\tmqtt\tinvid\tunit_guid\tmsg_id\ttext\tcontext\tclass\tlevel\tarea\taddr\tblock\ttype\tbit\tinvert_bit\n",
	)
	require.NoError(t, err)

	// Write a row with fewer fields than expected (missing fields)
	_, err = fmt.Fprintf(f, "1\t\tG-044322\tincomplete\n")
	require.NoError(t, err)

	require.NoError(t, f.Close())

	return f.Name()
}

func createEmptyTSV(t *testing.T) string {
	f, err := os.CreateTemp(t.TempDir(), "*.tsv")
	require.NoError(t, err)

	// Only write header with no data rows
	_, err = fmt.Fprintf(
		f,
		"n\tmqtt\tinvid\tunit_guid\tmsg_id\ttext\tcontext\tclass\tlevel\tarea\taddr\tblock\ttype\tbit\tinvert_bit\n",
	)
	require.NoError(t, err)

	require.NoError(t, f.Close())

	return f.Name()
}
