//go:build integration

package database_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/nais/contests/internal/database"
)

func TestPostgresConnectivity(t *testing.T) {
	dsn := os.Getenv("READWRITE_PGURL")
	u, err := url.Parse(dsn)
	if err != nil || dsn == "" {
		t.Fatal("set READWRITE_PGURL to a certificate-authenticated test database")
	}
	t.Setenv("PGHOST", "admin.invalid")
	t.Setenv("PGUSER", "admin")
	t.Setenv("PGSSLMODE", "disable")
	t.Setenv("PGSSLKEY", "/admin/tls.key")

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	var user string
	var canCreate bool
	if err := db.QueryRowContext(ctx, "SELECT current_user, has_schema_privilege(current_user, 'public', 'CREATE')").Scan(&user, &canCreate); err != nil {
		t.Fatal(err)
	}
	if user != u.User.Username() || canCreate {
		t.Fatalf("unexpected runtime privileges: user=%q, canCreate=%v", user, canCreate)
	}
	t.Run("success", func(t *testing.T) {
		for range 2 {
			if err := database.Check(ctx); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := database.Check(ctx); err == nil {
			t.Fatal("canceled check succeeded")
		}
	})
	t.Run("invalid client identity", func(t *testing.T) {
		invalid := *u
		invalid.User = url.User("certificate-does-not-match")
		t.Setenv("READWRITE_PGURL", invalid.String())
		if err := database.Check(ctx); err == nil {
			t.Fatal("invalid client identity was accepted")
		}
	})
}
