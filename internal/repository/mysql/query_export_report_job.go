package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/model"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
)

func (r *repo) QueryExportReportJob(ctx context.Context, filter repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
	query, args := r.buildExportReportJobQuery(filter)

	var reportJobs []*model.ExportReportJob
	if err := r.db.NamedSelectContext(ctx, &reportJobs, query, args); err != nil {
		err = errs.Wrap(constant.DBInternal, "query export report job", err)
		return nil, err
	}

	return reportJobs, nil
}

func (r *repo) buildExportReportJobQuery(filter repository.QueryExportReportJobFilter) (string, map[string]any) {
	conditionsQuery := make([]string, 0)
	queryArgsMap := make(map[string]any)

	if filter.JobID > 0 {
		conditionsQuery = append(conditionsQuery, "id = :job_id")
		queryArgsMap["job_id"] = filter.JobID
	}

	if filter.ShopID > 0 {
		conditionsQuery = append(conditionsQuery, "shop_id = :shop_id")
		queryArgsMap["shop_id"] = filter.ShopID
	}

	if filter.RequestID > 0 {
		conditionsQuery = append(conditionsQuery, "request_id = :request_id")
		queryArgsMap["request_id"] = filter.RequestID
	}

	if filter.LastExportReportJobID > 0 {
		conditionsQuery = append(conditionsQuery, "id > :last_export_report_job_id")
		queryArgsMap["last_export_report_job_id"] = filter.LastExportReportJobID
	}

	limit := filter.Limit
	if limit <= 0 || limit > repository.MaxQueryReportLimit {
		limit = repository.MaxQueryReportLimit
	}
	queryArgsMap["limit"] = limit

	baseQuery := selectExportReportJobsQuery
	if len(conditionsQuery) > 0 {
		baseQuery += fmt.Sprintf("\nWHERE %s", strings.Join(conditionsQuery, " AND "))
	}
	baseQuery += " ORDER BY id LIMIT :limit"

	return baseQuery, queryArgsMap
}
