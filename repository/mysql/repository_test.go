package mysql

import (
	"testing"

	"github.com/fikrimohammad/go-dev-sdk/db"
)

// stubDB is a non-nil db.DB used only to satisfy New's nil-check in tests that
// never execute queries.
type stubDB struct{ db.DB }

func newTestRepo(t *testing.T) *repo {
	t.Helper()
	return &repo{}
}

func TestNew_NilDB(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil database connection")
	}
}

func TestNew_Success(t *testing.T) {
	repo, err := New(&stubDB{})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
}
