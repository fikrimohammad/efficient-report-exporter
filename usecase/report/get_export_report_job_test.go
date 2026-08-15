package report

import (
	"context"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/mocks"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"go.uber.org/mock/gomock"
)

func TestGetExportReportJob_Success_Processing(t *testing.T) {
	ctrl := gomock.NewController(t)
	mysql := mocks.NewMockMySQL(ctrl)
	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		Return([]*model.ExportReportJob{
			{
				ID:           42,
				ShopID:       100,
				RequestID:    1,
				Status:       constant.ExportReportJobStatusProcessing,
				StartTime:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
				EndTime:      time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli(),
				CreationTime: time.Now().UnixMilli(),
				UpdateTime:   ptr(time.Now().UnixMilli()),
			},
		}, nil).
		AnyTimes()

	uc, err := New(mysql, mocks.NewMockMQ(ctrl), mocks.NewMockRedis(ctrl), mocks.NewMockS3(ctrl), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	result, err := uc.GetExportReportJob(context.Background(), usecase.GetExportReportJobParams{JobID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != constant.ExportReportJobStatusProcessing {
		t.Fatalf("expected status 'processing', got %s", result.Status)
	}
	if result.DownloadURL != "" {
		t.Fatalf("expected empty download URL for processing job, got %s", result.DownloadURL)
	}
}

func TestGetExportReportJob_Success_Completed(t *testing.T) {
	fileName := "report.csv"
	ctrl := gomock.NewController(t)
	mysql := mocks.NewMockMySQL(ctrl)
	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		Return([]*model.ExportReportJob{
			{
				ID:           42,
				ShopID:       100,
				RequestID:    1,
				Status:       constant.ExportReportJobStatusSuccess,
				StartTime:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
				EndTime:      time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli(),
				Extra:        model.ExportReportJobExtra{FileName: &fileName},
				CreationTime: time.Now().UnixMilli(),
				UpdateTime:   ptr(time.Now().UnixMilli()),
			},
		}, nil).
		AnyTimes()

	s3 := mocks.NewMockS3(ctrl)
	s3.EXPECT().
		GeneratePresignedDownloadURL(gomock.Any(), gomock.Any()).
		Return("https://s3.example.com/reports/report.csv?X-Amz-Signature=abc", nil).
		AnyTimes()

	uc, err := New(mysql, mocks.NewMockMQ(ctrl), mocks.NewMockRedis(ctrl), s3, newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	result, err := uc.GetExportReportJob(context.Background(), usecase.GetExportReportJobParams{JobID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != constant.ExportReportJobStatusSuccess {
		t.Fatalf("expected status 'success', got %s", result.Status)
	}
	if result.DownloadURL == "" {
		t.Fatal("expected non-empty download URL for completed job")
	}
}

func TestGetExportReportJob_Success_Failed(t *testing.T) {
	errMsg := "processing failed"
	ctrl := gomock.NewController(t)
	mysql := mocks.NewMockMySQL(ctrl)
	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		Return([]*model.ExportReportJob{
			{
				ID:           42,
				ShopID:       100,
				RequestID:    1,
				Status:       constant.ExportReportJobStatusFailed,
				StartTime:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
				EndTime:      time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli(),
				Extra:        model.ExportReportJobExtra{ErrMsg: &errMsg},
				CreationTime: time.Now().UnixMilli(),
				UpdateTime:   ptr(time.Now().UnixMilli()),
			},
		}, nil).
		AnyTimes()

	uc, err := New(mysql, mocks.NewMockMQ(ctrl), mocks.NewMockRedis(ctrl), mocks.NewMockS3(ctrl), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	result, err := uc.GetExportReportJob(context.Background(), usecase.GetExportReportJobParams{JobID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != constant.ExportReportJobStatusFailed {
		t.Fatalf("expected status 'failed', got %s", result.Status)
	}
	if result.ErrorMessage != "processing failed" {
		t.Fatalf("expected error message 'processing failed', got %s", result.ErrorMessage)
	}
}

func TestGetExportReportJob_JobNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mysql := mocks.NewMockMySQL(ctrl)
	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		Return([]*model.ExportReportJob{}, nil).
		AnyTimes()

	uc, err := New(mysql, mocks.NewMockMQ(ctrl), mocks.NewMockRedis(ctrl), mocks.NewMockS3(ctrl), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	_, err = uc.GetExportReportJob(context.Background(), usecase.GetExportReportJobParams{JobID: 999})
	if err == nil {
		t.Fatal("expected error for job not found")
	}
	if apperrors.CodeFromError(err) != apperrors.NotFound {
		t.Fatalf("expected NotFound code, got: %v", err)
	}
}

func TestGetExportReportJob_EmptyResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	mysql := mocks.NewMockMySQL(ctrl)
	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	uc, err := New(mysql, mocks.NewMockMQ(ctrl), mocks.NewMockRedis(ctrl), mocks.NewMockS3(ctrl), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	_, err = uc.GetExportReportJob(context.Background(), usecase.GetExportReportJobParams{JobID: 42})
	if err == nil {
		t.Fatal("expected error for empty query result")
	}
}

func TestGetExportReportJob_InvalidJobID(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, err := New(mocks.NewMockMySQL(ctrl), mocks.NewMockMQ(ctrl), mocks.NewMockRedis(ctrl), mocks.NewMockS3(ctrl), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	_, err = uc.GetExportReportJob(context.Background(), usecase.GetExportReportJobParams{JobID: 0})
	if err == nil {
		t.Fatal("expected error for zero job_id")
	}
}
