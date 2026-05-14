package application

import (
	"context"
	"database/sql"
	"log/slog"

	"log-parser/internal"
	"log-parser/internal/client/postgres"
	handler "log-parser/internal/delivery/http/handler"
	"log-parser/internal/migration"
	"log-parser/internal/repository"
	"log-parser/internal/service"
)

type Container struct {
	Config internal.Config
	Logger *slog.Logger
	DB     *sql.DB

	PublicServer *PublicServer

	TopologyRepo    *repository.TopologyRepository
	TopologyService *service.TopologyService
	TopologyHandler *handler.TopologyHandler
}

func NewContainer(cfg internal.Config, logger *slog.Logger) *Container {
	return &Container{
		Config: cfg,
		Logger: logger,
	}
}

func (c *Container) Init(ctx context.Context) error {
	db, err := postgres.NewDB(ctx, c.Config.DB)
	if err != nil {
		return err
	}
	c.DB = db

	if err := migration.NewMigrator(c.DB, c.Config.Migrations).Up(ctx); err != nil {
		return err
	}

	c.TopologyRepo = repository.NewTopologyRepository(c.DB)
	c.TopologyService = service.NewDefaultTopologyService(c.TopologyRepo, c.Config.DataDir, c.Logger)
	c.TopologyHandler = handler.NewTopologyHandler(c.TopologyService)

	mux := NewMux()
	c.TopologyHandler.Register(mux)

	c.PublicServer = NewPublicServer(c.Config, c.Logger)
	c.PublicServer.Configure(mux)
	return nil
}

func (c *Container) Close() error {
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}
