package report

import (
	"context"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
)

func TestGetExportReportJob_Success_Processing(t *testing.T) {
	mockMySQL := defaultMockMySQL()
	mockMySQL.queryExportReportJob = func(_ context.Context, filter repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return []*model.ExportReportJob{
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
		}, nil
	}

	uc, err := New(mockMySQL, defaultMockMQ(), defaultMockRedis(), defaultMockS3(), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	result, err := uc.GetExportReportJob(context.Background(), usecase.GetExportReportJobParams{JobID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "processing" {
		t.Fatalf("expected status 'processing', got %s", result.Status)
	}

	if result.DownloadURL != "" {
		t.Fatalf("expected empty download URL for processing job, got %s", result.DownloadURL)
	}
}

func TestGetExportReportJob_Success_Completed(t *testing.T) {
	fileName := "report.csv"
	mockMySQL := defaultMockMySQL()
	mockMySQL.queryExportReportJob = func(_ context.Context, filter repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return []*model.ExportReportJob{
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
		}, nil
	}

	uc, err := New(mockMySQL, defaultMockMQ(), defaultMockRedis(), defaultMockS3(), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	result, err := uc.GetExportReportJob(context.Background(), usecase.GetExportReportJobParams{JobID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "success" {
		t.Fatalf("expected status 'success', got %s", result.Status)
	}

	if result.DownloadURL == "" {
		t.Fatal("expected non-empty download URL for completed job")
	}
}

func TestGetExportReportJob_Success_Failed(t *testing.T) {
	errMsg := "processing failed"
	mockMySQL := defaultMockMySQL()
	mockMySQL.queryExportReportJob = func(_ context.Context, filter repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return []*model.ExportReportJob{
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
		}, nil
	}

	uc, err := New(mockMySQL, defaultMockMQ(), defaultMockRedis(), defaultMockS3(), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	result, err := uc.GetExportReportJob(context.Background(), usecase.GetExportReportJobParams{JobID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "failed" {
		t.Fatalf("expected status 'failed', got %s", result.Status)
	}

	if result.ErrorMessage != "processing failed" {
		t.Fatalf("expected error message 'processing failed', got %s", result.ErrorMessage)
	}
}

func TestGetExportReportJob_JobNotFound(t *testing.T) {
	mockMySQL := defaultMockMySQL()
	mockMySQL.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return []*model.ExportReportJob{}, nil
	}

	uc, err := New(mockMySQL, defaultMockMQ(), defaultMockRedis(), defaultMockS3(), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	_, err = uc.GetExportReportJob(context.Background(), usecase.GetExportReportJobParams{JobID: 999})
	if err == nil {
		t.Fatal("expected error for job not found")
	}

	if errs.CodeFromError(err) != errs.NotFound {
		t.Fatalf("expected NotFound code, got: %v", err)
	}
}

func TestGetExportReportJob_EmptyResult(t *testing.T) {
	mockMySQL := defaultMockMySQL()
	mockMySQL.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return nil, nil
	}

	uc, err := New(mockMySQL, defaultMockMQ(), defaultMockRedis(), defaultMockS3(), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	_, err = uc.GetExportReportJob(context.Background(), usecase.GetExportReportJobParams{JobID: 42})
	if err == nil {
		t.Fatal("expected error for empty query result")
	}
}

func TestGetExportReportJob_InvalidJobID(t *testing.T) {
	uc, err := New(defaultMockMySQL(), defaultMockMQ(), defaultMockRedis(), defaultMockS3(), newTestDynamicLoader(t))
	if err != nil {
		t.Fatalf("failed to create use case: %v", err)
	}

	_, err = uc.GetExportReportJob(context.Background(), usecase.GetExportReportJobParams{JobID: 0})
	if err == nil {
		t.Fatal("expected error for zero job_id")
	}
}
