package mq

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	mqmodel "github.com/fikrimohammad/efficient-report-exporter/internal/model/mq"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
)

func (r *repo) PublishExportReportProcessMsg(ctx context.Context, msg mqmodel.ExportReportProcessMessage) error {
	msgJSON, err := sonic.Marshal(msg)
	if err != nil {
		return errs.Wrap(constant.MQInternal, "marshal process message", err)
	}

	err = r.producer.PublishSync(
		ctx,
		string(constant.MQTopicReporting),
		string(constant.MQMsgTagExportReportProcess),
		msg.JobID,
		msgJSON,
	)
	if err != nil {
		return errs.Wrap(constant.MQInternal, "publish process message", err)
	}

	return nil
}
