package mq

import (
	"context"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

func (r *repo) PublishExportReportDoneMsg(ctx context.Context, msg model.ExportReportDoneMessage) error {
	msgJSON, err := sonic.Marshal(msg)
	if err != nil {
		return errs.Wrap(apperrors.MQInternal, "marshal done message", err)
	}

	err = r.producer.PublishSync(
		ctx,
		string(constant.MQTopicReporting),
		string(constant.MQMsgTagExportReportDone),
		strconv.FormatInt(msg.JobID, 10),
		msgJSON,
	)
	if err != nil {
		return errs.Wrap(apperrors.MQInternal, "publish done message", err)
	}

	return nil
}
