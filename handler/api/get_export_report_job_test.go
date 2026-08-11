package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
)

func TestGetExportReportJob_Success_Processing(t *testing.T) {
	mockUC := &mockReportUseCase{
		getExportReportJob: func(_ context.Context, params usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
			return &usecase.GetExportReportJobResult{
				JobID:        params.JobID,
				Status:       "processing",
				CreationTime: time.Now(),
				UpdateTime:   time.Now(),
			}, nil
		},
	}

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export/42")
	ts.readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_Success_Completed(t *testing.T) {
	mockUC := &mockReportUseCase{
		getExportReportJob: func(_ context.Context, params usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
			return &usecase.GetExportReportJobResult{
				JobID:        params.JobID,
				Status:       "success",
				DownloadURL:  "https://s3.example.com/reports/test.csv",
				CreationTime: time.Now(),
				UpdateTime:   time.Now(),
			}, nil
		},
	}

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export/42")
	ts.readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_Success_Failed(t *testing.T) {
	mockUC := &mockReportUseCase{
		getExportReportJob: func(_ context.Context, params usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
			return &usecase.GetExportReportJobResult{
				JobID:        params.JobID,
				Status:       "failed",
				ErrorMessage: "internal processing error",
				CreationTime: time.Now(),
				UpdateTime:   time.Now(),
			}, nil
		},
	}

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export/42")
	ts.readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_InvalidJobID(t *testing.T) {
	ts := setupTest(t, &mockReportUseCase{})
	resp := ts.get(t, "/v1/reports/export/abc")
	ts.readBody(t, resp)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_ZeroJobID(t *testing.T) {
	ts := setupTest(t, &mockReportUseCase{})
	resp := ts.get(t, "/v1/reports/export/0")
	ts.readBody(t, resp)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for zero job_id, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_JobNotFound(t *testing.T) {
	mockUC := &mockReportUseCase{
		getExportReportJob: func(_ context.Context, _ usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
			return nil, errs.New(errs.NotFound, "job not found")
		},
	}

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export/999")
	ts.readBody(t, resp)

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_InternalError(t *testing.T) {
	mockUC := &mockReportUseCase{
		getExportReportJob: func(_ context.Context, _ usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
			return nil, errors.New("database connection failed")
		},
	}

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export/1")
	ts.readBody(t, resp)

	if resp.StatusCode != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestGetExportReportJob_PresignError(t *testing.T) {
	mockUC := &mockReportUseCase{
		getExportReportJob: func(_ context.Context, _ usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
			return nil, errors.New("generate download url: signing failed")
		},
	}

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export/1")
	ts.readBody(t, resp)

	if resp.StatusCode != 500 {
		t.Fatalf("expected 500 for presign error, got %d", resp.StatusCode)
	}
}
