package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

func (r *repo) CountReport(ctx context.Context, filter repository.QueryReportFilter) (int64, error) {
	query, args := r.buildCountReportQuery(filter)

	var count int64
	if err := r.db.NamedGetContext(ctx, &count, query, args); err != nil {
		err = errs.Wrap(apperrors.DBInternal, "count report", err)
		return 0, err
	}

	return count, nil
}

func (r *repo) buildCountReportQuery(filter repository.QueryReportFilter) (string, map[string]any) {
	w := buildReportConditions(filter)

	if len(w.clauses) == 0 {
		return countReportQuery, w.args
	}

	return countReportQuery +
		fmt.Sprintf("\nWHERE %s", strings.Join(w.clauses, " AND ")), w.args
}
