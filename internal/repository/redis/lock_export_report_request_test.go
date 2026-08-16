package redis

import (
	"context"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/redis/go-redis/v9"
	"go.uber.org/mock/gomock"
)

func TestLockExportReportRequest_Success(t *testing.T) {
	mock := newMockRedisClient(t)
	mock.EXPECT().
		SetNX(gomock.Any(), "export_report_request:1", gomock.Any(), gomock.Any()).
		Return(redis.NewBoolResult(true, nil))

	repo := &repo{redisCli: mock}

	token, err := repo.LockExportReportRequest(context.Background(), repository.LockExportReportRequest{
		RequestID: 1,
		TTL:       5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty lock token")
	}
}

func TestLockExportReportRequest_AlreadyLocked(t *testing.T) {
	mock := newMockRedisClient(t)
	mock.EXPECT().
		SetNX(gomock.Any(), "export_report_request:1", gomock.Any(), gomock.Any()).
		Return(redis.NewBoolResult(false, nil))

	repo := &repo{redisCli: mock}

	_, err := repo.LockExportReportRequest(context.Background(), repository.LockExportReportRequest{
		RequestID: 1,
		TTL:       5 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error for already locked request, got nil")
	}
}

func TestLockExportReportRequest_RedisErrorOnSet(t *testing.T) {
	mock := newMockRedisClient(t)
	mock.EXPECT().
		SetNX(gomock.Any(), "export_report_request:1", gomock.Any(), gomock.Any()).
		Return(redis.NewBoolResult(false, redis.TxFailedErr))

	repo := &repo{redisCli: mock}

	_, err := repo.LockExportReportRequest(context.Background(), repository.LockExportReportRequest{
		RequestID: 1,
		TTL:       5 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error on redis SET NX failure, got nil")
	}
}
