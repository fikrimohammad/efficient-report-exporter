package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
	"go.uber.org/mock/gomock"
)

func TestGetExportReportJob_Success_Processing(t *testing.T) {
	mockUC := newMockReportUseCase(t)
	mockUC.EXPECT().
		GetExportReportJob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, params usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
			return &usecase.GetExportReportJobResult{
				JobID:        params.JobID,
				Status:       "processing",
				CreationTime: time.Now(),
				UpdateTime:   time.Now(),
			}, nil
		}).
		AnyTimes()

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export/42")
	ts.readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_Success_Completed(t *testing.T) {
	mockUC := newMockReportUseCase(t)
	mockUC.EXPECT().
		GetExportReportJob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, params usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
			return &usecase.GetExportReportJobResult{
				JobID:        params.JobID,
				Status:       "success",
				DownloadURL:  "https://s3.example.com/reports/test.csv",
				CreationTime: time.Now(),
				UpdateTime:   time.Now(),
			}, nil
		}).
		AnyTimes()

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export/42")
	ts.readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_Success_Failed(t *testing.T) {
	mockUC := newMockReportUseCase(t)
	mockUC.EXPECT().
		GetExportReportJob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, params usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
			return &usecase.GetExportReportJobResult{
				JobID:        params.JobID,
				Status:       "failed",
				ErrorMessage: "internal processing error",
				CreationTime: time.Now(),
				UpdateTime:   time.Now(),
			}, nil
		}).
		AnyTimes()

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export/42")
	ts.readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_InvalidJobID(t *testing.T) {
	ts := setupTest(t, newMockReportUseCase(t))
	resp := ts.get(t, "/v1/reports/export/abc")
	ts.readBody(t, resp)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_ZeroJobID(t *testing.T) {
	ts := setupTest(t, newMockReportUseCase(t))
	resp := ts.get(t, "/v1/reports/export/0")
	ts.readBody(t, resp)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for zero job_id, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_JobNotFound(t *testing.T) {
	mockUC := newMockReportUseCase(t)
	mockUC.EXPECT().
		GetExportReportJob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
			return nil, errs.New(constant.NotFound, "job not found")
		}).
		AnyTimes()

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export/999")
	ts.readBody(t, resp)

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_InternalError(t *testing.T) {
	mockUC := newMockReportUseCase(t)
	mockUC.EXPECT().
		GetExportReportJob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
			return nil, errors.New("database connection failed")
		}).
		AnyTimes()

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export/1")
	ts.readBody(t, resp)

	if resp.StatusCode != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_PresignError(t *testing.T) {
	mockUC := newMockReportUseCase(t)
	mockUC.EXPECT().
		GetExportReportJob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
			return nil, errors.New("generate download url: signing failed")
		}).
		AnyTimes()

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export/1")
	ts.readBody(t, resp)

	if resp.StatusCode != 500 {
		t.Fatalf("expected 500 for presign error, got %d", resp.StatusCode)
	}
}
