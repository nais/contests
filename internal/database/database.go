// Package database checks Nais Postgres connectivity using runtime credentials.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

// Check opens a new TLS connection and executes SELECT 1 without changing data.
func Check(ctx context.Context) (err error) {
	dsn := os.Getenv("READWRITE_PGURL")
	if dsn == "" {
		return errors.New("READWRITE_PGURL is required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("open Postgres: %w", err)
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	var result int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
		return fmt.Errorf("query Postgres: %w", err)
	}
	return nil
}
