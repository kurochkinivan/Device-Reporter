package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/kurochkinivan/device_reporter/internal/domain"
)

type Scanner struct {
	log           *slog.Logger
	watchDir      string
	scanInterval  time.Duration
	files         chan<- string
	filesProvider FilesProvider
	fileUpdater   FileUpdater
}

func NewScanner(
	log *slog.Logger,
	watchDir string,
	scanInterval time.Duration,
	files chan<- string,
	filesProvider FilesProvider,
	fileUpdater FileUpdater,
) *Scanner {
	return &Scanner{
		log:           log,
		watchDir:      watchDir,
		scanInterval:  scanInterval,
		files:         files,
		filesProvider: filesProvider,
		fileUpdater:   fileUpdater,
	}
}

func (s *Scanner) Run(ctx context.Context) error {
	defer close(s.files)

	ticker := time.NewTicker(s.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.log.DebugContext(ctx, "scan cycle started")

			err := s.scanFiles(ctx)
			if err != nil {
				s.log.ErrorContext(ctx, "failed to scan files", slog.String("err", err.Error()))
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Scanner) scanFiles(ctx context.Context) error {
	filesMap, err := s.extractFilesFromDB(ctx)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(s.watchDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %q", s.watchDir)
	}

	for _, entry := range entries {
		err := s.processEntry(ctx, entry, filesMap)

		if err != nil {
			s.log.ErrorContext(ctx, "failed process entry, skipping file",
				slog.String("filename", entry.Name()),
				slog.String("err", err.Error()),
			)
			continue
		}
	}

	return nil
}

func (s *Scanner) extractFilesFromDB(ctx context.Context) (map[string]domain.Status, error) {
	files, err := s.filesProvider.Files(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get files: %w", err)
	}

	filesMap := make(map[string]domain.Status, len(files))
	for _, file := range files {
		filesMap[file.Name] = file.Status
	}

	return filesMap, nil
}

func (s *Scanner) processEntry(ctx context.Context, entry os.DirEntry, filesMap map[string]domain.Status) error {
	if entry.IsDir() {
		return nil
	}

	status, ok := filesMap[entry.Name()]
	if ok && status != domain.StatusPending {
		return nil
	}

	err := s.fileUpdater.UpdateOrCreateFile(ctx, &domain.File{
		Name:   entry.Name(),
		Status: domain.StatusProcessing,
	})
	if err != nil {
		return fmt.Errorf("failed to update file status: %w", err)
	}

	s.log.DebugContext(ctx, "updated file status to processing", slog.String("filename", entry.Name()))

	s.files <- filepath.Join(s.watchDir, entry.Name())

	return nil
}
