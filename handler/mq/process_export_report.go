package mq

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
)

func (h *Handler) ProcessExportReport(ctx context.Context, msgBody []byte) error {
	var msg model.ExportReportProcessMessage
	if err := sonic.Unmarshal(msgBody, &msg); err != nil {
		return errs.Wrap(errs.InvalidArgument, "unmarshal process export report message", err)
	}

	p := usecase.ProcessExportReportParams{
		JobID: msg.JobID,
	}

	if err := h.reportUseCase.ProcessExportReport(ctx, p); err != nil {
		return err
	}

	return nil
}
