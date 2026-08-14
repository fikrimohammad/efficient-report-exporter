package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

func (r *repo) QueryReport(ctx context.Context, filter repository.QueryReportFilter) ([]*model.Report, error) {
	query, args := r.buildReportQuery(ctx, filter)

	var reports []*model.Report
	if err := r.db.NamedSelectContext(ctx, &reports, query, args); err != nil {
		err = errs.Wrap(errs.DBInternal, "query report", err)
		return nil, err
	}

	return reports, nil
}

func (r *repo) buildReportQuery(_ context.Context, filter repository.QueryReportFilter) (string, map[string]interface{}) {
	conditionsQuery, queryArgsMap := buildReportConditions(filter)

	limitQuery := ""
	if filter.Limit > 0 {
		if filter.Limit > repository.MaxQueryReportLimit {
			filter.Limit = repository.MaxQueryReportLimit
		}

		limitQuery = limitQuery + " LIMIT :limit"
		queryArgsMap["limit"] = filter.Limit
	}

	if len(conditionsQuery) == 0 {
		return selectReportQuery, nil
	}

	query := selectReportQuery +
		fmt.Sprintf("\nWHERE %s", strings.Join(conditionsQuery, " AND ")) +
		" ORDER BY id ASC" +
		limitQuery

	return query, queryArgsMap
}

// buildReportConditions builds the shared WHERE conditions (shop_id and
// order_settlement_time range, plus the id cursor) used by both the report
// select and count queries, so the two can't drift.
func buildReportConditions(filter repository.QueryReportFilter) ([]string, map[string]interface{}) {
	var (
		conditionsQuery = make([]string, 0)
		queryArgsMap    = make(map[string]any)
	)

	if filter.ShopID != nil && *filter.ShopID > 0 {
		conditionsQuery = append(conditionsQuery, "shop_id = :shop_id")
		queryArgsMap["shop_id"] = *filter.ShopID
	}

	if filter.OrderSettlementTimeRange != nil {
		var (
			orderSettlementTimeRangeQuery = make([]string, 0)
			startTime                     = filter.OrderSettlementTimeRange.StartTime
			endTime                       = filter.OrderSettlementTimeRange.EndTime
		)

		if startTime != nil && !startTime.IsZero() {
			orderSettlementTimeRangeQuery = append(orderSettlementTimeRangeQuery, "order_settlement_time >= :order_settlement_time_start_time")
			queryArgsMap["order_settlement_time_start_time"] = *startTime
		}

		if endTime != nil && !endTime.IsZero() {
			orderSettlementTimeRangeQuery = append(orderSettlementTimeRangeQuery, "order_settlement_time <= :order_settlement_time_end_time")
			queryArgsMap["order_settlement_time_end_time"] = *endTime
		}

		if len(orderSettlementTimeRangeQuery) > 0 {
			conditionsQuery = append(conditionsQuery, fmt.Sprintf("(%s)", strings.Join(orderSettlementTimeRangeQuery, " AND ")))
		}
	}

	if filter.LastReportID > 0 {
		conditionsQuery = append(conditionsQuery, "id > :last_report_id")
		queryArgsMap["last_report_id"] = filter.LastReportID
	}

	return conditionsQuery, queryArgsMap
}
