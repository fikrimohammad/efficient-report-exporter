package redis

import (
	"context"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

func TestUnlockExportReportProcess_Success(t *testing.T) {
	mock := newMockRedisClient()
	repo := &repo{redisCli: mock}

	mock.set("export_report_job:42", "1")

	err := repo.UnlockExportReportProcess(context.Background(), repository.UnlockExportReportProcess{
		JobID: 42,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnlockExportReportProcess_NotLocked(t *testing.T) {
	mock := newMockRedisClient()
	repo := &repo{redisCli: mock}

	err := repo.UnlockExportReportProcess(context.Background(), repository.UnlockExportReportProcess{
		JobID: 42,
	})
	if err != nil {
		t.Fatalf("unexpected error for non-locked key: %v", err)
	}
}
