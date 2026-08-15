package mq

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

func (h *Handler) ProcessExportReport(ctx context.Context, msgBody []byte) error {
	var msg model.ExportReportProcessMessage
	if err := sonic.Unmarshal(msgBody, &msg); err != nil {
		return errs.Wrap(apperrors.InvalidArgument, "unmarshal process export report message", err)
	}

	p := usecase.ProcessExportReportParams{
		JobID: msg.JobID,
	}

	if err := h.reportUseCase.ProcessExportReport(ctx, p); err != nil {
		return err
	}

	return nil
}
