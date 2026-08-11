package report

import (
	"context"
	"time"

	"github.com/fikrimohammad/go-dev-sdk/errs"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
)

func (u *useCase) ListExportReportJobs(ctx context.Context, params usecase.ListExportReportJobsParams) (*usecase.ListExportReportJobsResult, error) {
	if params.ShopID == 0 {
		return nil, errs.New(errs.InvalidArgument, "shop_id is required")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = constant.DefaultListExportReportJobsLimit
	}
	if limit > constant.MaxListExportReportJobsLimit {
		limit = constant.MaxListExportReportJobsLimit
	}

	jobs, err := u.mySQLRepository.QueryExportReportJob(ctx, repository.QueryExportReportJobFilter{
		ShopID:                params.ShopID,
		Limit:                 limit + 1,
		LastExportReportJobID: params.PageToken,
	})
	if err != nil {
		return nil, err
	}

	result := &usecase.ListExportReportJobsResult{
		Jobs: make([]usecase.ExportReportJobSummary, 0, len(jobs)),
	}

	for i, job := range jobs {
		if i >= limit {
			result.NextPageToken = job.ID
			break
		}

		var updateTime time.Time
		if job.UpdateTime != nil {
			updateTime = time.UnixMilli(*job.UpdateTime)
		}

		result.Jobs = append(result.Jobs, usecase.ExportReportJobSummary{
			JobID:        job.ID,
			Status:       string(job.Status),
			StartTime:    time.UnixMilli(job.StartTime),
			EndTime:      time.UnixMilli(job.EndTime),
			CreationTime: time.UnixMilli(job.CreationTime),
			UpdateTime:   updateTime,
		})
	}

	return result, nil
}
