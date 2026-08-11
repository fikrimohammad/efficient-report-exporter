package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/fikrimohammad/go-dev-sdk/errs"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
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
	var (
		conditionsQuery = make([]string, 0)
		limitQuery      = ""
		queryArgsMap    = make(map[string]any)
		baseQuery       = selectReportQuery
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

	if filter.Limit > 0 {
		if filter.Limit > repository.MaxQueryReportLimit {
			filter.Limit = repository.MaxQueryReportLimit
		}

		limitQuery = limitQuery + " LIMIT :limit"
		queryArgsMap["limit"] = filter.Limit
	}

	if len(conditionsQuery) == 0 {
		return baseQuery, nil
	}

	baseQuery = baseQuery +
		fmt.Sprintf("\nWHERE %s", strings.Join(conditionsQuery, " AND ")) +
		" ORDER BY id ASC" +
		limitQuery

	return baseQuery, queryArgsMap
}
