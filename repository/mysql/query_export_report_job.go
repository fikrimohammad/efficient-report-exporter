package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

func (r *repo) QueryExportReportJob(ctx context.Context, filter repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
	query, args := r.buildExportReportJobQuery(ctx, filter)

	var reportJobs []*model.ExportReportJob
	if err := r.db.NamedSelectContext(ctx, &reportJobs, query, args); err != nil {
		err = errs.Wrap(errs.DBInternal, "query export report job", err)
		return nil, err
	}

	return reportJobs, nil
}

func (r *repo) buildExportReportJobQuery(_ context.Context, filter repository.QueryExportReportJobFilter) (string, map[string]interface{}) {
	var (
		conditionsQuery = make([]string, 0)
		limitQuery      string
		queryArgsMap    = make(map[string]interface{})
		baseQuery       = selectExportReportJobsQuery
	)

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

	limitQuery = " LIMIT :limit"
	queryArgsMap["limit"] = repository.MaxQueryReportLimit
	if filter.Limit > 0 && filter.Limit <= repository.MaxQueryReportLimit {
		limitQuery = " LIMIT :limit"
		queryArgsMap["limit"] = filter.Limit
	}

	if len(conditionsQuery) == 0 {
		return baseQuery, nil
	}

	baseQuery = baseQuery +
		fmt.Sprintf("\nWHERE %s", strings.Join(conditionsQuery, " AND ")) +
		" ORDER BY id" +
		limitQuery

	return baseQuery, queryArgsMap
}
