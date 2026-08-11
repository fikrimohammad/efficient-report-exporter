package mq

import (
	"context"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
)

func (r *repo) PublishExportReportProcessMsg(ctx context.Context, msg model.ExportReportProcessMessage) error {
	msgJSON, err := sonic.Marshal(msg)
	if err != nil {
		return errs.Wrap(errs.MQInternal, "marshal process message", err)
	}

	err = r.producer.PublishSync(
		ctx,
		string(constant.MQTopicReporting),
		string(constant.MQMsgTagExportReportProcess),
		strconv.FormatInt(msg.JobID, 10),
		msgJSON,
	)
	if err != nil {
		return errs.Wrap(errs.MQInternal, "publish process message", err)
	}

	return nil
}
