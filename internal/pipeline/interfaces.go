package pipeline

import (
	"context"

	"github.com/kurochkinivan/device_reporter/internal/domain"
)

type FilesProvider interface {
	Files(ctx context.Context) ([]*domain.File, error)
}

type FileUpdater interface {
	UpdateOrCreateFile(ctx context.Context, file *domain.File) error
}

type DevicesSaver interface {
	SaveDevices(ctx context.Context, devices ...*domain.Device) error
}

type Transactor interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type ReportGenerator interface {
	GenerateReport(outputPath, unitGUID, sourceFile string, devices []*domain.Device) error
}
