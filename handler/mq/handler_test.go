package mq

import (
	"context"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

type mockReportUseCase struct {
	requestExportReport  func(ctx context.Context, params usecase.RequestExportReportParams) (*usecase.RequestExportReportResult, error)
	processExportReport  func(ctx context.Context, params usecase.ProcessExportReportParams) error
	getExportReportJob   func(ctx context.Context, params usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error)
	listExportReportJobs func(ctx context.Context, params usecase.ListExportReportJobsParams) (*usecase.ListExportReportJobsResult, error)
}

func (m *mockReportUseCase) RequestExportReport(ctx context.Context, params usecase.RequestExportReportParams) (*usecase.RequestExportReportResult, error) {
	return m.requestExportReport(ctx, params)
}

func (m *mockReportUseCase) ProcessExportReport(ctx context.Context, params usecase.ProcessExportReportParams) error {
	return m.processExportReport(ctx, params)
}

func (m *mockReportUseCase) GetExportReportJob(ctx context.Context, params usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
	return m.getExportReportJob(ctx, params)
}

func (m *mockReportUseCase) ListExportReportJobs(ctx context.Context, params usecase.ListExportReportJobsParams) (*usecase.ListExportReportJobsResult, error) {
	return m.listExportReportJobs(ctx, params)
}

func TestNewHandler_NilUseCase(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil use case")
	}

	var e *errs.Error
	if !errs.As(err, &e) {
		t.Fatalf("expected *errs.Error, got %T", err)
	}
	if e.Code() != errs.Internal {
		t.Errorf("expected code Internal, got %v", e.Code())
	}
}
