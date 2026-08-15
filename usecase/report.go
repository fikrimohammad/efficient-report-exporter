package usecase

import (
	"context"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

type RequestExportReportParams struct {
	RequestID int64     `json:"request_id"`
	ShopID    int64     `json:"shop_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

func (p RequestExportReportParams) Validate() error {
	if p.ShopID == 0 {
		return errs.New(apperrors.InvalidArgument, "shop_id is required")
	}

	if p.StartTime.IsZero() {
		return errs.New(apperrors.InvalidArgument, "start_time is required")
	}

	if p.EndTime.IsZero() {
		return errs.New(apperrors.InvalidArgument, "end_time is required")
	}

	if p.StartTime.After(p.EndTime) {
		return errs.New(apperrors.InvalidArgument, "start_time is after end_time")
	}

	if p.EndTime.Sub(p.StartTime) > constant.MaxExportTimeRange {
		return errs.New(apperrors.InvalidArgument, "time range must not exceed 90 days")
	}

	return nil
}

type RequestExportReportResult struct {
	JobID int64 `json:"job_id,string"`
}

type ProcessExportReportParams struct {
	JobID int64 `json:"job_id"`
}

type GetExportReportJobParams struct {
	JobID int64 `json:"job_id"`
}

type GetExportReportJobResult struct {
	JobID        int64                          `json:"job_id,string"`
	Status       constant.ExportReportJobStatus `json:"status"`
	DownloadURL  string                         `json:"download_url,omitempty"`
	ErrorMessage string                         `json:"error_message,omitempty"`
	CreationTime time.Time                      `json:"creation_time"`
	UpdateTime   time.Time                      `json:"update_time"`
}

type ListExportReportJobsParams struct {
	ShopID int64 `json:"shop_id"`
	// PageToken is the ID cursor (the last job id of the previous page), despite
	// the "token" name carried over from the API contract.
	PageToken int64 `json:"page_token"`
	Limit     int   `json:"limit"`
}

type ListExportReportJobsResult struct {
	Jobs          []ExportReportJobSummary `json:"jobs"`
	NextPageToken int64                    `json:"next_page_token"`
}

type ExportReportJobSummary struct {
	JobID        int64                          `json:"job_id,string"`
	Status       constant.ExportReportJobStatus `json:"status"`
	StartTime    time.Time                      `json:"start_time"`
	EndTime      time.Time                      `json:"end_time"`
	CreationTime time.Time                      `json:"creation_time"`
	UpdateTime   time.Time                      `json:"update_time"`
}

type Report interface {
	RequestExportReport(ctx context.Context, params RequestExportReportParams) (*RequestExportReportResult, error)
	ProcessExportReport(ctx context.Context, params ProcessExportReportParams) error
	GetExportReportJob(ctx context.Context, params GetExportReportJobParams) (*GetExportReportJobResult, error)
	ListExportReportJobs(ctx context.Context, params ListExportReportJobsParams) (*ListExportReportJobsResult, error)
}
