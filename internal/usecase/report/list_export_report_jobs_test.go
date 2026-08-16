package report

import (
	"context"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/mock"
	"github.com/fikrimohammad/efficient-report-exporter/internal/model"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/fikrimohammad/efficient-report-exporter/internal/usecase"
	"go.uber.org/mock/gomock"
)

func TestListExportReportJobs_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mysql := mock.NewMockMySQL(ctrl)
	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		Return([]*model.ExportReportJob{
			{
				ID:           101,
				ShopID:       100,
				RequestID:    1,
				Status:       constant.ExportReportJobStatusSuccess,
				StartTime:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
				EndTime:      time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli(),
				CreationTime: time.Now().UnixMilli(),
				UpdateTime:   ptr(time.Now().UnixMilli()),
			},
			{
				ID:           102,
				ShopID:       100,
				RequestID:    2,
				Status:       constant.ExportReportJobStatusProcessing,
				StartTime:    time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC).UnixMilli(),
				EndTime:      time.Date(2025, 1, 4, 0, 0, 0, 0, time.UTC).UnixMilli(),
				CreationTime: time.Now().UnixMilli(),
				UpdateTime:   ptr(time.Now().UnixMilli()),
			},
		}, nil).
		AnyTimes()

	uc, err := New(mysql, mock.NewMockMQ(ctrl), mock.NewMockRedis(ctrl), mock.NewMockS3(ctrl), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	result, err := uc.ListExportReportJobs(context.Background(), usecase.ListExportReportJobsParams{
		ShopID: 100,
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result.Jobs))
	}
	if result.NextPageToken != 0 {
		t.Fatalf("expected no next page token, got %d", result.NextPageToken)
	}
}

func TestListExportReportJobs_Success_Pagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	mysql := mock.NewMockMySQL(ctrl)
	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		Return([]*model.ExportReportJob{
			{ID: 101, Status: constant.ExportReportJobStatusSuccess},
			{ID: 102, Status: constant.ExportReportJobStatusProcessing},
			{ID: 103, Status: constant.ExportReportJobStatusFailed},
		}, nil).
		AnyTimes()

	uc, err := New(mysql, mock.NewMockMQ(ctrl), mock.NewMockRedis(ctrl), mock.NewMockS3(ctrl), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	result, err := uc.ListExportReportJobs(context.Background(), usecase.ListExportReportJobsParams{
		ShopID: 100,
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result.Jobs))
	}
	if result.NextPageToken == 0 {
		t.Fatal("expected next page token, got 0")
	}
}

func TestListExportReportJobs_Success_EmptyResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	mysql := mock.NewMockMySQL(ctrl)
	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		Return([]*model.ExportReportJob{}, nil).
		AnyTimes()

	uc, err := New(mysql, mock.NewMockMQ(ctrl), mock.NewMockRedis(ctrl), mock.NewMockS3(ctrl), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	result, err := uc.ListExportReportJobs(context.Background(), usecase.ListExportReportJobsParams{
		ShopID: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(result.Jobs))
	}
}

func TestListExportReportJobs_MissingShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, err := New(mock.NewMockMySQL(ctrl), mock.NewMockMQ(ctrl), mock.NewMockRedis(ctrl), mock.NewMockS3(ctrl), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	_, err = uc.ListExportReportJobs(context.Background(), usecase.ListExportReportJobsParams{
		ShopID: 0,
	})
	if err == nil {
		t.Fatal("expected error for missing shop_id")
	}
}

func TestListExportReportJobs_NilResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	mysql := mock.NewMockMySQL(ctrl)
	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	uc, err := New(mysql, mock.NewMockMQ(ctrl), mock.NewMockRedis(ctrl), mock.NewMockS3(ctrl), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	result, err := uc.ListExportReportJobs(context.Background(), usecase.ListExportReportJobsParams{
		ShopID: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(result.Jobs))
	}
}

func TestListExportReportJobs_DefaultLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	mysql := mock.NewMockMySQL(ctrl)

	var capturedLimit int
	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, filter repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
			capturedLimit = filter.Limit
			return []*model.ExportReportJob{}, nil
		}).
		AnyTimes()

	uc, err := New(mysql, mock.NewMockMQ(ctrl), mock.NewMockRedis(ctrl), mock.NewMockS3(ctrl), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	_, _ = uc.ListExportReportJobs(context.Background(), usecase.ListExportReportJobsParams{
		ShopID: 100,
		Limit:  0,
	})

	expectedLimit := constant.DefaultListExportReportJobsLimit + 1
	if capturedLimit != expectedLimit {
		t.Fatalf("expected limit %d, got %d", expectedLimit, capturedLimit)
	}
}

func TestListExportReportJobs_CapLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	mysql := mock.NewMockMySQL(ctrl)

	var capturedLimit int
	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, filter repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
			capturedLimit = filter.Limit
			return []*model.ExportReportJob{}, nil
		}).
		AnyTimes()

	uc, err := New(mysql, mock.NewMockMQ(ctrl), mock.NewMockRedis(ctrl), mock.NewMockS3(ctrl), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	_, _ = uc.ListExportReportJobs(context.Background(), usecase.ListExportReportJobsParams{
		ShopID: 100,
		Limit:  200,
	})

	expectedLimit := constant.MaxListExportReportJobsLimit + 1
	if capturedLimit != expectedLimit {
		t.Fatalf("expected limit %d, got %d", expectedLimit, capturedLimit)
	}
}
