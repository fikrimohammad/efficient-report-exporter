package mq

import (
	"context"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
)

func (r *repo) PublishExportReportDoneMsg(ctx context.Context, msg model.ExportReportDoneMessage) error {
	msgJSON, err := sonic.Marshal(msg)
	if err != nil {
		return errs.Wrap(errs.MQInternal, "marshal done message", err)
	}

	err = r.producer.PublishSync(
		ctx,
		string(constant.MQTopicReporting),
		string(constant.MQMsgTagExportReportDone),
		strconv.FormatInt(msg.JobID, 10),
		msgJSON,
	)
	if err != nil {
		return errs.Wrap(errs.MQInternal, "publish done message", err)
	}

	return nil
}
