package application

import (
	"context"
	"database/sql"
	"log/slog"

	"topology-parser/internal"
	"topology-parser/internal/client/postgres"
	"topology-parser/internal/migration"
)

type Container struct {
	Config internal.Config
	Logger *slog.Logger
	DB     *sql.DB

	PublicServer *PublicServer
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

	c.PublicServer = NewPublicServer(c.Config, c.Logger)
	c.PublicServer.Configure(NewMux())
	return nil
}

func (c *Container) Close() error {
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}
