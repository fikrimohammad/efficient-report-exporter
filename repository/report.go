package repository

import (
	"context"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/go-typedpipe"
)

type ReportMySQL interface {
	AsyncQueryReport(ctx context.Context, filter QueryReportFilter) (typedpipe.Reader[model.Report], error)
}

type QueryReportFilter struct {
	ShopID                   *int64
	OrderSettlementTimeRange *QueryReportTimeRange
}

type QueryReportTimeRange struct {
	StartTime *time.Time
	EndTime   *time.Time
}
