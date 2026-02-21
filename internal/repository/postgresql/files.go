package postgresql

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kurochkinivan/device_reporter/internal/domain"
)

const TableFiles = "files"

type FilesRepository struct {
	pool *pgxpool.Pool
	qb   sq.StatementBuilderType
}

func NewFilesRepository(pool *pgxpool.Pool) *FilesRepository {
	return &FilesRepository{
		pool: pool,
		qb:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *FilesRepository) Files(ctx context.Context) ([]*domain.File, error) {
	db := extractDB(ctx, r.pool)

	sql, args, err := r.qb.
		Select(
			"name",
			"status",
			"processed_at",
			"error_message",
		).
		From(TableFiles).
		ToSql()
	if err != nil {
		return nil, createQueryError(err)
	}

	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, executeQueryError(err)
	}

	files, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[domain.File])
	if err != nil {
		return nil, collectRowsError(err)
	}

	return files, nil
}

func (r *FilesRepository) UpdateOrCreateFile(ctx context.Context, file *domain.File) error {
	db := extractDB(ctx, r.pool)

	sql, args, err := r.qb.
		Insert(TableFiles).
		Columns(
			"name",
			"status",
			"error_message",
			"processed_at",
		).
		Values(
			file.Name,
			file.Status,
			file.ErrorMessage,
			file.ProcessedAt,
		).
		Suffix(`ON CONFLICT (name) DO UPDATE SET 
			status = EXCLUDED.status, 
			error_message = EXCLUDED.error_message, 
			processed_at = EXCLUDED.processed_at
		`).
		ToSql()
	if err != nil {
		return createQueryError(err)
	}

	_, err = db.Exec(ctx, sql, args...)
	if err != nil {
		return executeQueryError(err)
	}

	return nil
}

func (r *FilesRepository) ResetProcessingFiles(ctx context.Context) error {
	db := extractDB(ctx, r.pool)

	sql, args, err := r.qb.
		Update(TableFiles).
		Set("status", domain.StatusPending).
		Where(sq.Eq{"status": domain.StatusProcessing}).
		ToSql()
	if err != nil {
		return createQueryError(err)
	}

	_, err = db.Exec(ctx, sql, args...)
	if err != nil {
		return executeQueryError(err)
	}

	return nil
}
