package report

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/go-typedpipe"
	"github.com/jmoiron/sqlx"
)

func (r *repo) AsyncQueryReport(ctx context.Context, filter repository.QueryReportFilter) (typedpipe.Reader[model.Report], error) {
	rows, err := r.queryReport(ctx, filter)
	if err != nil {
		return nil, err
	}

	reportsDataStreamWriter, reportsDataStreamReader, err := typedpipe.New[model.Report]()
	if err != nil {
		return nil, err
	}

	go func(goCtx context.Context, goRows *sqlx.Rows, w typedpipe.Writer[model.Report]) {
		defer func() {
			if r := recover(); r != nil {
				w.CloseWithError(fmt.Errorf("%v", r))
			}

			goRows.Close()
			w.Close()
		}()

		for goRows.Next() {
			if goRows.Err() != nil {
				w.CloseWithError(goRows.Err())
				return
			}

			var report model.Report
			if scanErr := goRows.StructScan(&report); scanErr != nil {
				w.CloseWithError(scanErr)
				return
			}

			writerErr := w.Write(goCtx, report)
			if writerErr != nil && !errors.Is(writerErr, typedpipe.ErrPipeClosed) {
				w.CloseWithError(writerErr)
				return
			}
		}
	}(ctx, rows, reportsDataStreamWriter)

	return reportsDataStreamReader, nil
}

func (r *repo) queryReport(ctx context.Context, filter repository.QueryReportFilter) (*sqlx.Rows, error) {
	query, args, err := r.buildReportQuery(ctx, filter)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *repo) buildReportQuery(_ context.Context, filter repository.QueryReportFilter) (string, []interface{}, error) {
	var (
		conditionsQuery = make([]string, 0)
		conditionsArgs  = make(map[string]interface{})
		baseQuery       = selectReportQuery
	)

	if filter.ShopID != nil && *filter.ShopID > 0 {
		conditionsQuery = append(conditionsQuery, "shop_id = :shop_id")
		conditionsArgs["shop_id"] = *filter.ShopID
	}

	if filter.OrderSettlementTimeRange != nil {
		var (
			orderSettlementTimeRangeQuery = make([]string, 0)
			startTime                     = filter.OrderSettlementTimeRange.StartTime
			endTime                       = filter.OrderSettlementTimeRange.EndTime
		)

		if startTime != nil && !startTime.IsZero() {
			orderSettlementTimeRangeQuery = append(orderSettlementTimeRangeQuery, "order_settlement_time >= :order_settlement_time_start_time")
			conditionsArgs["order_settlement_start_time"] = *startTime
		}

		if endTime != nil && !endTime.IsZero() {
			orderSettlementTimeRangeQuery = append(orderSettlementTimeRangeQuery, "order_settlement_time <= :order_settlement_time_end_time")
			conditionsArgs["order_settlement_end_time"] = *endTime
		}

		if len(orderSettlementTimeRangeQuery) > 0 {
			conditionsQuery = append(conditionsQuery, fmt.Sprintf("(%s)", strings.Join(orderSettlementTimeRangeQuery, " AND ")))
		}
	}

	if len(conditionsQuery) == 0 {
		return baseQuery, nil, nil
	}

	baseQuery = baseQuery + fmt.Sprintf("\nWHERE %s", strings.Join(conditionsQuery, " AND "))
	query, args, err := r.db.BindNamed(baseQuery, conditionsArgs)
	if err != nil {
		return baseQuery, nil, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return baseQuery, nil, err
	}

	query = r.db.Rebind(query)
	return query, args, nil
}
