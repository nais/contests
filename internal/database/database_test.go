package database_test

import (
	"strings"
	"testing"

	"github.com/nais/contests/internal/database"
)

func TestCheckDoesNotFallBackToAdminCredentials(t *testing.T) {
	t.Setenv("READWRITE_PGURL", "")
	t.Setenv("PGURL", "postgres://admin@localhost/app")
	err := database.Check(t.Context())
	if err == nil || !strings.Contains(err.Error(), "READWRITE_PGURL") {
		t.Fatalf("error = %v, want missing runtime credentials", err)
	}
}
