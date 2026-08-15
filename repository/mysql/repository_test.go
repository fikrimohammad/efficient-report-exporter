package mysql

import (
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/internal/mocks"
	"go.uber.org/mock/gomock"
)

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
	db := mocks.NewMockDB(gomock.NewController(t))
	repo, err := New(db)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
}
