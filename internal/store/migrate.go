package store

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func Migrate(dsn string, migrationsFS fs.FS) error {
	src, err := iofs.New(migrationsFS, "db/migrations")
	if err != nil {
		return fmt.Errorf("open migrations fs: %w", err)
	}
	defer src.Close()

	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("new migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
