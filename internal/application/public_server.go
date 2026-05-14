package application

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"topology-parser/internal"
	transport "topology-parser/internal/delivery/http"
	"topology-parser/internal/delivery/http/middleware"
)

const APIV1Prefix = "/api/v1"

var ErrServerClosed = http.ErrServerClosed

type PublicServer struct {
	cfg    internal.Config
	logger *slog.Logger
	server *http.Server
}

func NewPublicServer(cfg internal.Config, logger *slog.Logger) *PublicServer {
	return &PublicServer{
		cfg:    cfg,
		logger: logger,
	}
}

func (s *PublicServer) Configure(handler http.Handler) {
	chain := middleware.Recover(s.logger)(middleware.Logging(s.logger)(handler))
	s.server = &http.Server{
		Addr:    ":" + s.cfg.Port,
		Handler: chain,
	}
}

func (s *PublicServer) Start() error {
	if s.server == nil {
		return errors.New("public server is not configured")
	}
	s.logger.Info("starting public server", "address", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *PublicServer) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	transport.RegisterHealthRoutes(mux)
	return mux
}
