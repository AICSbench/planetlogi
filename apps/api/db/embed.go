package dbmigrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var MigrationsFS embed.FS

// Up applies all embedded goose SQL migrations against the provided sql.DB handle.
func Up(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("run goose migrations: %w", err)
	}
	return nil
}
