package redis

import (
	"context"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

func TestUnlockExportReportRequest_Success(t *testing.T) {
	mock := newMockRedisClient()
	repo := &repo{redisCli: mock}

	mock.set("export_report_request:1", "1")

	err := repo.UnlockExportReportRequest(context.Background(), repository.UnlockExportReportRequest{
		RequestID: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnlockExportReportRequest_NotLocked(t *testing.T) {
	mock := newMockRedisClient()
	repo := &repo{redisCli: mock}

	err := repo.UnlockExportReportRequest(context.Background(), repository.UnlockExportReportRequest{
		RequestID: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error for non-locked key: %v", err)
	}
}
