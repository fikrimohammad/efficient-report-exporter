package usecase

import (
	"context"
	"io"
	"time"
)

type ExportReportParams struct {
	ShopID    int64
	StartTime time.Time
	EndTime   time.Time
}

type ExportReportResult struct {
	FileName string
	File     io.ReadCloser
}

type Report interface {
	ExportReport(ctx context.Context, params ExportReportParams) (*ExportReportResult, error)
}
