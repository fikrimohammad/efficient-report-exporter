package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/usecase"
)

func TestRequestExportReport_Success(t *testing.T) {
	mockUC := &mockReportUseCase{
		requestExportReport: func(_ context.Context, _ usecase.RequestExportReportParams) (*usecase.RequestExportReportResult, error) {
			return &usecase.RequestExportReportResult{JobID: 42}, nil
		},
	}

	ts := setupTest(t, mockUC)
	resp := ts.post(t, "/v1/reports/export", `{"request_id":"1","shop_id":"100","start_time":"2025-01-01T00:00:00Z","end_time":"2025-01-02T00:00:00Z"}`)
	ts.readBody(t, resp)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRequestExportReport_MissingShopID(t *testing.T) {
	ts := setupTest(t, &mockReportUseCase{})
	resp := ts.post(t, "/v1/reports/export", `{"request_id":"1","start_time":"2025-01-01T00:00:00Z","end_time":"2025-01-02T00:00:00Z"}`)
	ts.readBody(t, resp)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing shop_id, got %d", resp.StatusCode)
	}
}

func TestRequestExportReport_InvalidStartTime(t *testing.T) {
	ts := setupTest(t, &mockReportUseCase{})
	resp := ts.post(t, "/v1/reports/export", `{"request_id":"1","shop_id":"100","start_time":"invalid","end_time":"2025-01-02T00:00:00Z"}`)
	ts.readBody(t, resp)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for invalid start_time, got %d", resp.StatusCode)
	}
}

func TestRequestExportReport_InvalidEndTime(t *testing.T) {
	ts := setupTest(t, &mockReportUseCase{})
	resp := ts.post(t, "/v1/reports/export", `{"request_id":"1","shop_id":"100","start_time":"2025-01-01T00:00:00Z","end_time":"invalid"}`)
	ts.readBody(t, resp)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for invalid end_time, got %d", resp.StatusCode)
	}
}

func TestRequestExportReport_UseCaseError(t *testing.T) {
	mockUC := &mockReportUseCase{
		requestExportReport: func(_ context.Context, _ usecase.RequestExportReportParams) (*usecase.RequestExportReportResult, error) {
			return nil, errors.New("internal error")
		},
	}

	ts := setupTest(t, mockUC)
	resp := ts.post(t, "/v1/reports/export", `{"request_id":"1","shop_id":"100","start_time":"2025-01-01T00:00:00Z","end_time":"2025-01-02T00:00:00Z"}`)
	ts.readBody(t, resp)

	if resp.StatusCode != 500 {
		t.Fatalf("expected 500 for use case error, got %d", resp.StatusCode)
	}
}

func TestRequestExportReport_InvalidBody(t *testing.T) {
	ts := setupTest(t, &mockReportUseCase{})
	resp := ts.post(t, "/v1/reports/export", `not json`)
	ts.readBody(t, resp)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for invalid body, got %d", resp.StatusCode)
	}
}

func TestRequestExportReport_JSONTags(t *testing.T) {
	params := usecase.RequestExportReportParams{
		RequestID: 1,
		ShopID:    100,
	}
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded usecase.RequestExportReportParams
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.RequestID != 1 || decoded.ShopID != 100 {
		t.Fatal("JSON round-trip failed")
	}
}
