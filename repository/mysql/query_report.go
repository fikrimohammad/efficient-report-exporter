package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

func (r *repo) QueryReport(ctx context.Context, filter repository.QueryReportFilter) ([]*model.Report, error) {
	query, args := r.buildReportQuery(filter)

	var reports []*model.Report
	if err := r.db.NamedSelectContext(ctx, &reports, query, args); err != nil {
		err = errs.Wrap(apperrors.DBInternal, "query report", err)
		return nil, err
	}

	return reports, nil
}

func (r *repo) buildReportQuery(filter repository.QueryReportFilter) (string, map[string]any) {
	w := buildReportConditions(filter)

	limitQuery := ""
	if filter.Limit > 0 {
		if filter.Limit > repository.MaxQueryReportLimit {
			filter.Limit = repository.MaxQueryReportLimit
		}

		limitQuery = limitQuery + " LIMIT :limit"
		w.args["limit"] = filter.Limit
	}

	if len(w.clauses) == 0 {
		return selectReportQuery, nil
	}

	query := selectReportQuery +
		fmt.Sprintf("\nWHERE %s", strings.Join(w.clauses, " AND ")) +
		" ORDER BY order_settlement_time ASC, id ASC" +
		limitQuery

	return query, w.args
}

// reportConditions holds the WHERE clauses and named query args shared by the
// report select and count queries, so the two can't drift.
type reportConditions struct {
	clauses []string
	args    map[string]any
}

// buildReportConditions builds the shared WHERE conditions (shop_id and
// order_settlement_time range, plus the keyset cursor).
func buildReportConditions(filter repository.QueryReportFilter) reportConditions {
	w := reportConditions{
		clauses: make([]string, 0),
		args:    make(map[string]any),
	}

	if filter.ShopID != nil && *filter.ShopID > 0 {
		w.clauses = append(w.clauses, "shop_id = :shop_id")
		w.args["shop_id"] = *filter.ShopID
	}

	if filter.OrderSettlementTimeRange != nil {
		var (
			rangeClauses = make([]string, 0)
			startTime    = filter.OrderSettlementTimeRange.StartTime
			endTime      = filter.OrderSettlementTimeRange.EndTime
		)

		if startTime != nil && !startTime.IsZero() {
			rangeClauses = append(rangeClauses, "order_settlement_time >= :order_settlement_time_start_time")
			w.args["order_settlement_time_start_time"] = *startTime
		}

		if endTime != nil && !endTime.IsZero() {
			rangeClauses = append(rangeClauses, "order_settlement_time <= :order_settlement_time_end_time")
			w.args["order_settlement_time_end_time"] = *endTime
		}

		if len(rangeClauses) > 0 {
			w.clauses = append(w.clauses, fmt.Sprintf("(%s)", strings.Join(rangeClauses, " AND ")))
		}
	}

	if filter.LastOrderSettlementTime != nil && !filter.LastOrderSettlementTime.IsZero() {
		w.clauses = append(w.clauses,
			"(order_settlement_time > :last_order_settlement_time OR (order_settlement_time = :last_order_settlement_time AND id > :last_report_id))")
		w.args["last_order_settlement_time"] = *filter.LastOrderSettlementTime
		w.args["last_report_id"] = filter.LastReportID
	}

	return w
}
