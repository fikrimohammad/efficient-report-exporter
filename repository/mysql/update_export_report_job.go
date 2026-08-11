package mysql

import (
	"context"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

func (r *repo) UpdateExportReportJob(ctx context.Context, params repository.UpdateExportReportJobParams) error {
	query, args := r.buildUpdateExportReportJobQuery(ctx, params)
	if _, err := r.db.NamedExecContext(ctx, query, args); err != nil {
		err = errs.Wrap(errs.DBInternal, "update export report job", err)
		return err
	}
	return nil
}

func (r *repo) buildUpdateExportReportJobQuery(_ context.Context, params repository.UpdateExportReportJobParams) (string, map[string]interface{}) {
	queryArgsMap := map[string]interface{}{
		"id":          params.JobID,
		"status":      params.Status,
		"extra":       params.Extra,
		"update_time": time.Now().UnixMilli(),
	}

	return updateExportReportJobQuery, queryArgsMap
}
