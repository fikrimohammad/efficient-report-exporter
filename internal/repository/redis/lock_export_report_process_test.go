package redis

import (
	"context"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/redis/go-redis/v9"
	"go.uber.org/mock/gomock"
)

func TestLockExportReportProcess_Success(t *testing.T) {
	mock := newMockRedisClient(t)
	mock.EXPECT().
		SetNX(gomock.Any(), "export_report_job:42", gomock.Any(), gomock.Any()).
		Return(redis.NewBoolResult(true, nil))

	repo := &repo{redisCli: mock}

	token, err := repo.LockExportReportProcess(context.Background(), repository.LockExportReportProcess{
		JobID: 42,
		TTL:   1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty lock token")
	}
}

func TestLockExportReportProcess_AlreadyLocked(t *testing.T) {
	mock := newMockRedisClient(t)
	mock.EXPECT().
		SetNX(gomock.Any(), "export_report_job:42", gomock.Any(), gomock.Any()).
		Return(redis.NewBoolResult(false, nil))

	repo := &repo{redisCli: mock}

	_, err := repo.LockExportReportProcess(context.Background(), repository.LockExportReportProcess{
		JobID: 42,
		TTL:   1 * time.Minute,
	})
	if err == nil {
		t.Fatal("expected error for already locked job, got nil")
	}
}

func TestLockExportReportProcess_RedisErrorOnSet(t *testing.T) {
	mock := newMockRedisClient(t)
	mock.EXPECT().
		SetNX(gomock.Any(), "export_report_job:42", gomock.Any(), gomock.Any()).
		Return(redis.NewBoolResult(false, redis.TxFailedErr))

	repo := &repo{redisCli: mock}

	_, err := repo.LockExportReportProcess(context.Background(), repository.LockExportReportProcess{
		JobID: 42,
		TTL:   1 * time.Minute,
	})
	if err == nil {
		t.Fatal("expected error on redis SET failure, got nil")
	}
}
