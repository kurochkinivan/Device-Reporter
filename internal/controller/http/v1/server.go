package v1

import (
	"context"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/kurochkinivan/device_reporter/internal/config"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(cfg config.HTTP, devicesRepo DevicesRepository) *Server {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	h := NewDevicesHandler(devicesRepo)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/devices/{unit_guid}", h.GetDevicesByUnitGUID)
	})

	return &Server{
		httpServer: &http.Server{
			Addr:         net.JoinHostPort(cfg.Host, cfg.Port),
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
			Handler:      r,
		},
	}
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
