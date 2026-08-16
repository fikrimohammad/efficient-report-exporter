package mq

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/mock"
	mqmodel "github.com/fikrimohammad/efficient-report-exporter/internal/model/mq"
	"github.com/fikrimohammad/efficient-report-exporter/internal/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
	"go.uber.org/mock/gomock"
)

func TestProcessExportReport_Success(t *testing.T) {
	mockUC := mock.NewMockReportUseCase(gomock.NewController(t))
	mockUC.EXPECT().
		ProcessExportReport(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ usecase.ProcessExportReportParams) error {
			return nil
		}).
		AnyTimes()

	h := &Handler{reportUseCase: mockUC}

	msg := mqmodel.ExportReportProcessMessage{JobID: "42"}
	body, err := sonic.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if err := h.ProcessExportReport(context.Background(), body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessExportReport_UnmarshalError(t *testing.T) {
	h := &Handler{}

	err := h.ProcessExportReport(context.Background(), []byte("{invalid json"))
	if err == nil {
		t.Fatal("expected unmarshal error")
	}

	var e *errs.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *errs.Error, got %T", err)
	}
	if e.Code() != constant.InvalidArgument {
		t.Errorf("expected code InvalidArgument, got %v", e.Code())
	}
}

func TestProcessExportReport_UseCaseError(t *testing.T) {
	mockUC := mock.NewMockReportUseCase(gomock.NewController(t))
	mockUC.EXPECT().
		ProcessExportReport(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ usecase.ProcessExportReportParams) error {
			return errors.New("processing failed")
		}).
		AnyTimes()

	h := &Handler{reportUseCase: mockUC}

	msg := mqmodel.ExportReportProcessMessage{JobID: "42"}
	body, err := sonic.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	err = h.ProcessExportReport(context.Background(), body)
	if err == nil {
		t.Fatal("expected error")
	}

	if err.Error() != "processing failed" {
		t.Fatalf("expected 'processing failed', got %v", err)
	}
}
