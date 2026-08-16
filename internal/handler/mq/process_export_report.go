package mq

import (
	"context"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	mqmodel "github.com/fikrimohammad/efficient-report-exporter/internal/model/mq"
	"github.com/fikrimohammad/efficient-report-exporter/internal/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
)

func (h *Handler) ProcessExportReport(ctx context.Context, msgBody []byte) error {
	var msg mqmodel.ExportReportProcessMessage
	if err := sonic.Unmarshal(msgBody, &msg); err != nil {
		return errs.Wrap(constant.InvalidArgument, "unmarshal process export report message", err)
	}

	jobID, err := strconv.ParseInt(msg.JobID, 10, 64)
	if err != nil {
		return errs.Wrap(constant.InvalidArgument, "invalid job_id format", err)
	}

	p := usecase.ProcessExportReportParams{
		JobID: jobID,
	}

	if err := h.reportUseCase.ProcessExportReport(ctx, p); err != nil {
		return err
	}

	return nil
}
