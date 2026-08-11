package redis

import (
	"context"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/redis/go-redis/v9"
)

func TestLockExportReportRequest_Success(t *testing.T) {
	mock := newMockRedisClient()
	repo := &repo{redisCli: mock}

	err := repo.LockExportReportRequest(context.Background(), repository.LockExportReportRequest{
		RequestID: 1,
		TTL:       5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLockExportReportRequest_AlreadyLocked(t *testing.T) {
	mock := newMockRedisClient()
	repo := &repo{redisCli: mock}

	mock.set("export_report_request:1", "1")

	err := repo.LockExportReportRequest(context.Background(), repository.LockExportReportRequest{
		RequestID: 1,
		TTL:       5 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error for already locked request, got nil")
	}
}

func TestLockExportReportRequest_RedisErrorOnSet(t *testing.T) {
	mock := newMockRedisClient()
	mock.setErr = redis.TxFailedErr
	repo := &repo{redisCli: mock}

	err := repo.LockExportReportRequest(context.Background(), repository.LockExportReportRequest{
		RequestID: 1,
		TTL:       5 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error on redis SET NX failure, got nil")
	}
}
