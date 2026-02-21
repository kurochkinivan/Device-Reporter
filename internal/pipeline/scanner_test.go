package pipeline_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kurochkinivan/device_reporter/internal/domain"
	"github.com/kurochkinivan/device_reporter/internal/pipeline"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestScanner_Run_HappyPath(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	tmpDir := t.TempDir()

	// Тестовый файл
	f, err := os.CreateTemp(tmpDir, "*.tsv")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	filename := f.Name()

	scanInterval := 1 * time.Millisecond
	files := make(chan string, 1)

	// Файла еще нет в БД
	filesProvider := NewMockFilesProvider(t)
	filesProvider.EXPECT().
		Files(mock.Anything).
		Return([]*domain.File{}, nil)

	// Ожидается запрос на изменение нашего файла
	filesStatusUpdater := NewMockFileUpdater(t)
	filesStatusUpdater.EXPECT().
		UpdateOrCreateFile(mock.Anything, mock.MatchedBy(func(f *domain.File) bool {
			return f.Name == filepath.Base(filename) && f.Status == domain.StatusProcessing
		})).
		Return(nil)

	scanner := pipeline.NewScanner(log, tmpDir, scanInterval, files, filesProvider, filesStatusUpdater)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- scanner.Run(ctx)
	}()

	// Ждем файл в канале
	select {
	case got := <-files:
		assert.Equal(t, filename, got)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: file was not sent to channel")
	}

	// Отмена контекста, сканнер должен остановиться
	cancel()

	// Ждем завершения горутины
	select {
	case err := <-errChan:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: scanner did not stop")
	}
}

func TestScanner_Run_FileInDBWithPendingStatus(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	tmpDir := t.TempDir()

	// Тестовый файл
	f, err := os.CreateTemp(tmpDir, "*.tsv")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	filename := f.Name()

	scanInterval := 1 * time.Millisecond
	files := make(chan string, 1)

	// Файла в БД со статусом Pending
	filesProvider := NewMockFilesProvider(t)
	filesProvider.EXPECT().
		Files(mock.Anything).
		Return([]*domain.File{{Name: filepath.Base(filename), Status: domain.StatusPending}}, nil)

	// Ожидается запрос на изменение нашего файла
	fileUpdater := NewMockFileUpdater(t)
	fileUpdater.EXPECT().
		UpdateOrCreateFile(mock.Anything, mock.MatchedBy(func(f *domain.File) bool {
			return f.Name == filepath.Base(filename) && f.Status == domain.StatusProcessing
		})).
		Return(nil)

	scanner := pipeline.NewScanner(log, tmpDir, scanInterval, files, filesProvider, fileUpdater)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- scanner.Run(ctx)
	}()

	// Ждем файл в канале
	select {
	case got := <-files:
		assert.Equal(t, filename, got)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: file was not sent to channel")
	}

	// Отмена контекста, сканнер должен остановиться
	cancel()

	// Ждем завершения горутины
	select {
	case err := <-errChan:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: scanner did not stop")
	}
}

func TestScanner_Run_FilesAlreadyInDB(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.DiscardHandler)

	tmpDir := t.TempDir()

	// Тестовые файлы
	filenames := make([]string, 0, 3)
	for range 3 {
		f, err := os.CreateTemp(tmpDir, "*.tsv")
		require.NoError(t, err)
		require.NoError(t, f.Close())

		filenames = append(filenames, filepath.Base(f.Name()))
	}

	scanInterval := 1 * time.Millisecond
	files := make(chan string, 1)

	// Файлы уже в БД со статусами НЕ pending
	filesProvider := NewMockFilesProvider(t)
	filesProvider.EXPECT().
		Files(mock.Anything).
		Return([]*domain.File{
			{Name: filenames[0], Status: domain.StatusProcessing},
			{Name: filenames[1], Status: domain.StatusDone},
			{Name: filenames[2], Status: domain.StatusError},
		}, nil)

	// Не ожидается запросов на изменение файла
	filesStatusUpdater := NewMockFileUpdater(t)

	scanner := pipeline.NewScanner(log, tmpDir, scanInterval, files, filesProvider, filesStatusUpdater)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- scanner.Run(ctx)
	}()

	// Ждем окончания сканирования
	select {
	case got := <-files:
		t.Fatalf("didn't expect files, got %q", got)
	case <-time.After(scanInterval * 10):
	}

	// Отмена контекста, сканнер должен остановиться
	cancel()

	// Ждем завершения горутины
	select {
	case err := <-errChan:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timeout: scanner did not stop")
	}
}
