package postgres

import (
	"context"
	"database/sql"
	"time"

	"topology-parser/internal"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewDB(ctx context.Context, cfg internal.PostgresConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}
