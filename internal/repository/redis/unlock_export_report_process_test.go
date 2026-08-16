package redis

import (
	"context"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/redis/go-redis/v9"
	"go.uber.org/mock/gomock"
)

func TestUnlockExportReportProcess_Success(t *testing.T) {
	mock := newMockRedisClient(t)
	mock.EXPECT().
		Get(gomock.Any(), "export_report_job:42").
		Return(redis.NewStringResult("owner-token", nil))
	mock.EXPECT().
		Del(gomock.Any(), "export_report_job:42").
		Return(redis.NewIntResult(1, nil))

	repo := &repo{redisCli: mock}

	err := repo.UnlockExportReportProcess(context.Background(), repository.UnlockExportReportProcess{
		JobID: 42,
		Token: "owner-token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnlockExportReportProcess_WrongTokenDoesNotRelease(t *testing.T) {
	mock := newMockRedisClient(t)
	mock.EXPECT().
		Get(gomock.Any(), "export_report_job:42").
		Return(redis.NewStringResult("other-owner-token", nil))

	repo := &repo{redisCli: mock}

	err := repo.UnlockExportReportProcess(context.Background(), repository.UnlockExportReportProcess{
		JobID: 42,
		Token: "owner-token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnlockExportReportProcess_NotLocked(t *testing.T) {
	mock := newMockRedisClient(t)
	mock.EXPECT().
		Get(gomock.Any(), "export_report_job:42").
		Return(redis.NewStringResult("", redis.Nil))

	repo := &repo{redisCli: mock}

	err := repo.UnlockExportReportProcess(context.Background(), repository.UnlockExportReportProcess{
		JobID: 42,
		Token: "owner-token",
	})
	if err != nil {
		t.Fatalf("unexpected error for non-locked key: %v", err)
	}
}
