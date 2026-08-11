package redis

import (
	"context"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/redis/go-redis/v9"
)

func TestLockExportReportProcess_Success(t *testing.T) {
	mock := newMockRedisClient()
	repo := &repo{redisCli: mock}

	err := repo.LockExportReportProcess(context.Background(), repository.LockExportReportProcess{
		JobID: 42,
		TTL:   1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLockExportReportProcess_AlreadyLocked(t *testing.T) {
	mock := newMockRedisClient()
	repo := &repo{redisCli: mock}

	mock.set("export_report_job:42", "1")

	err := repo.LockExportReportProcess(context.Background(), repository.LockExportReportProcess{
		JobID: 42,
		TTL:   1 * time.Minute,
	})
	if err == nil {
		t.Fatal("expected error for already locked job, got nil")
	}
}

func TestLockExportReportProcess_RedisErrorOnSet(t *testing.T) {
	mock := newMockRedisClient()
	mock.setErr = redis.TxFailedErr
	repo := &repo{redisCli: mock}

	err := repo.LockExportReportProcess(context.Background(), repository.LockExportReportProcess{
		JobID: 42,
		TTL:   1 * time.Minute,
	})
	if err == nil {
		t.Fatal("expected error on redis SET failure, got nil")
	}
}
