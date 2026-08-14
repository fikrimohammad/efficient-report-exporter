package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

func (r *repo) CountReport(ctx context.Context, filter repository.QueryReportFilter) (int64, error) {
	query, args := r.buildCountReportQuery(ctx, filter)

	var count int64
	if err := r.db.NamedGetContext(ctx, &count, query, args); err != nil {
		err = errs.Wrap(errs.DBInternal, "count report", err)
		return 0, err
	}

	return count, nil
}

func (r *repo) buildCountReportQuery(_ context.Context, filter repository.QueryReportFilter) (string, map[string]interface{}) {
	conditionsQuery, queryArgsMap := buildReportConditions(filter)

	if len(conditionsQuery) == 0 {
		return countReportQuery, queryArgsMap
	}

	return countReportQuery +
		fmt.Sprintf("\nWHERE %s", strings.Join(conditionsQuery, " AND ")), queryArgsMap
}
