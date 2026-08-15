package mysql

import (
	"context"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

func (r *repo) UpdateExportReportJob(ctx context.Context, params repository.UpdateExportReportJobParams) error {
	query, args := r.buildUpdateExportReportJobQuery(params)
	if _, err := r.db.NamedExecContext(ctx, query, args); err != nil {
		err = errs.Wrap(apperrors.DBInternal, "update export report job", err)
		return err
	}
	return nil
}

func (r *repo) buildUpdateExportReportJobQuery(params repository.UpdateExportReportJobParams) (string, map[string]any) {
	queryArgsMap := map[string]any{
		"id":          params.JobID,
		"status":      params.Status,
		"extra":       params.Extra,
		"update_time": time.Now().UnixMilli(),
	}

	return updateExportReportJobQuery, queryArgsMap
}
