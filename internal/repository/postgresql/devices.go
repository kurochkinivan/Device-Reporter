package postgresql

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kurochkinivan/device_reporter/internal/domain"
)

const TableDevices = "devices"

type DevicesRepository struct {
	pool *pgxpool.Pool
	qb   sq.StatementBuilderType
}

func NewDevicesRepository(pool *pgxpool.Pool) *DevicesRepository {
	return &DevicesRepository{
		pool: pool,
		qb:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *DevicesRepository) DevicesByGUID(
	ctx context.Context,
	guid string,
	limit, offset uint64,
) ([]*domain.Device, int, error) {
	db := extractDB(ctx, r.pool)

	sql, args, err := r.qb.
		Select("COUNT(*)").
		From(TableDevices).
		Where(sq.Eq{"unit_guid": guid}).
		ToSql()
	if err != nil {
		return nil, -1, createQueryError(err)
	}

	var total int
	if err := db.QueryRow(ctx, sql, args...).Scan(&total); err != nil {
		return nil, -1, scanRowError(err)
	}

	sql, args, err = r.qb.
		Select(
			"n",
			"mqtt",
			"inv_id",
			"unit_guid",
			"msg_id",
			"text",
			"context",
			"class",
			"level",
			"area",
			"addr",
			"block",
			"type",
			"bit",
			"invert_bit",
		).
		From(TableDevices).
		Where(sq.Eq{"unit_guid": guid}).
		OrderBy("n ASC").
		Limit(limit).
		Offset(offset).
		ToSql()
	if err != nil {
		return nil, -1, createQueryError(err)
	}

	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, -1, executeQueryError(err)
	}

	devices, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[domain.Device])
	if err != nil {
		return nil, -1, collectRowsError(err)
	}

	return devices, total, nil
}

func (r *DevicesRepository) SaveDevices(ctx context.Context, devices ...*domain.Device) error {
	db := extractDB(ctx, r.pool)

	copied, err := db.CopyFrom(ctx, pgx.Identifier{TableDevices}, []string{
		"n",
		"mqtt",
		"inv_id",
		"unit_guid",
		"msg_id",
		"text",
		"context",
		"class",
		"level",
		"area",
		"addr",
		"block",
		"type",
		"bit",
		"invert_bit",
	}, pgx.CopyFromSlice(len(devices), func(i int) ([]any, error) {
		return []any{
			devices[i].N,
			devices[i].MQTT,
			devices[i].InvID,
			devices[i].UnitGUID,
			devices[i].MsgID,
			devices[i].Text,
			devices[i].Context,
			devices[i].Class,
			devices[i].Level,
			devices[i].Area,
			devices[i].Addr,
			devices[i].Block,
			devices[i].Type,
			devices[i].Bit,
			devices[i].InvertBit,
		}, nil
	}))
	if err != nil {
		return fmt.Errorf("failed to save devices: %w", err)
	}

	if copied != int64(len(devices)) {
		return fmt.Errorf("failed to save devices: copied %d rows, expected %d", copied, len(devices))
	}

	return nil
}
