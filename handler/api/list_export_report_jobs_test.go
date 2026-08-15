package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"go.uber.org/mock/gomock"
)

func TestListExportReportJobs_Success(t *testing.T) {
	mockUC := newMockReport(t)
	mockUC.EXPECT().
		ListExportReportJobs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, params usecase.ListExportReportJobsParams) (*usecase.ListExportReportJobsResult, error) {
			return &usecase.ListExportReportJobsResult{
				Jobs: []usecase.ExportReportJobSummary{
					{
						JobID:        100,
						Status:       "success",
						StartTime:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
						EndTime:      time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
						CreationTime: time.Now(),
						UpdateTime:   time.Now(),
					},
				},
				NextPageToken: 0,
			}, nil
		}).
		AnyTimes()

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export?shop_id=100")
	ts.readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestListExportReportJobs_Success_EmptyResult(t *testing.T) {
	mockUC := newMockReport(t)
	mockUC.EXPECT().
		ListExportReportJobs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ usecase.ListExportReportJobsParams) (*usecase.ListExportReportJobsResult, error) {
			return &usecase.ListExportReportJobsResult{
				Jobs:          []usecase.ExportReportJobSummary{},
				NextPageToken: 0,
			}, nil
		}).
		AnyTimes()

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export?shop_id=100")
	ts.readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestListExportReportJobs_MissingShopID(t *testing.T) {
	ts := setupTest(t, newMockReport(t))
	resp := ts.get(t, "/v1/reports/export")
	ts.readBody(t, resp)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestListExportReportJobs_InvalidShopID(t *testing.T) {
	ts := setupTest(t, newMockReport(t))
	resp := ts.get(t, "/v1/reports/export?shop_id=abc")
	ts.readBody(t, resp)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestListExportReportJobs_UseCaseError(t *testing.T) {
	mockUC := newMockReport(t)
	mockUC.EXPECT().
		ListExportReportJobs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ usecase.ListExportReportJobsParams) (*usecase.ListExportReportJobsResult, error) {
			return nil, errors.New("internal error")
		}).
		AnyTimes()

	ts := setupTest(t, mockUC)
	resp := ts.get(t, "/v1/reports/export?shop_id=100")
	ts.readBody(t, resp)

	if resp.StatusCode != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestListExportReportJobs_InvalidPageToken(t *testing.T) {
	ts := setupTest(t, newMockReport(t))
	resp := ts.get(t, "/v1/reports/export?shop_id=100&page_token=abc")
	ts.readBody(t, resp)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestListExportReportJobs_InvalidLimit(t *testing.T) {
	ts := setupTest(t, newMockReport(t))
	resp := ts.get(t, "/v1/reports/export?shop_id=100&limit=abc")
	ts.readBody(t, resp)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
