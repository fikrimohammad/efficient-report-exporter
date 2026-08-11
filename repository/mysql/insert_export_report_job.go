package mysql

import (
	"context"
	"time"

	snowflake "github.com/godruoyi/go-snowflake"

	"github.com/fikrimohammad/go-dev-sdk/errs"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

func (r *repo) InsertExportReportJob(ctx context.Context, params repository.InsertExportReportJobParams) (*model.ExportReportJob, error) {
	jobID, err := snowflake.NextID()
	if err != nil {
		err = errs.Wrap(errs.Internal, "generate snowflake id", err)
		return nil, err
	}

	query, args := r.buildInsertExportReportJobQuery(ctx, int64(jobID), params)

	if _, err := r.db.NamedExecContext(ctx, query, args); err != nil {
		err = errs.Wrap(errs.DBInternal, "insert export report job", err)
		return nil, err
	}

	exportReportJob := &model.ExportReportJob{
		ID:        int64(jobID),
		ShopID:    params.ShopID,
		Status:    params.Status,
		StartTime: params.StartTime,
		EndTime:   params.EndTime,
		Extra:     params.Extra,
	}

	return exportReportJob, nil
}

func (r *repo) buildInsertExportReportJobQuery(_ context.Context, jobID int64, params repository.InsertExportReportJobParams) (string, map[string]interface{}) {
	queryArgsMap := map[string]interface{}{
		"id":            jobID,
		"request_id":    params.RequestID,
		"shop_id":       params.ShopID,
		"start_time":    params.StartTime,
		"end_time":      params.EndTime,
		"status":        params.Status,
		"extra":         params.Extra,
		"creation_time": time.Now().UnixMilli(),
		"update_time":   nil,
	}

	return insertExportReportJobQuery, queryArgsMap
}
