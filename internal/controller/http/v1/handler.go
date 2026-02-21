package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kurochkinivan/device_reporter/internal/domain"
)

type DevicesHandler struct {
	devicesRepository DevicesRepository
}

type DevicesRepository interface {
	DevicesByGUID(ctx context.Context, guid string, limit, offset uint64) ([]*domain.Device, int, error)
}

func NewDevicesHandler(devicesRepository DevicesRepository) *DevicesHandler {
	return &DevicesHandler{
		devicesRepository: devicesRepository,
	}
}

type GetDevicesByUnitGUIDResponse struct {
	Devices    []*domain.Device `json:"devices"`
	Pagination Pagination       `json:"pagination"`
}

func (h *DevicesHandler) GetDevicesByUnitGUID(w http.ResponseWriter, r *http.Request) {
	unitGUID := chi.URLParam(r, "unit_guid")

	page, limit, err := h.parsePagination(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	offset := (page - 1) * limit

	devices, total, err := h.devicesRepository.DevicesByGUID(r.Context(), unitGUID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(GetDevicesByUnitGUIDResponse{
		Devices: devices,
		Pagination: Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: (total + int(limit) - 1) / int(limit),
		},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(data)
}

func (h *DevicesHandler) parsePagination(r *http.Request) (page uint64, limit uint64, err error) {
	page, limit = 1, 10

	if p := r.URL.Query().Get("page"); p != "" {
		page, err = strconv.ParseUint(p, 10, 64)
		if err != nil || page == 0 {
			return 0, 0, errors.New("invalid page")
		}
	}

	if l := r.URL.Query().Get("limit"); l != "" {
		limit, err = strconv.ParseUint(l, 10, 64)
		if err != nil || limit < 1 || limit > 100 {
			return 0, 0, errors.New("invalid limit, must be in [1;100]")
		}
	}

	return page, limit, nil
}
