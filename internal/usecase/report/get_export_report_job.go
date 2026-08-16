package report

import (
	"context"
	"fmt"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/fikrimohammad/efficient-report-exporter/internal/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
)

func (u *useCase) GetExportReportJob(ctx context.Context, params usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
	if params.JobID == 0 {
		return nil, errs.New(constant.InvalidArgument, "job_id is required")
	}

	jobs, err := u.mysqlRepository.QueryExportReportJob(ctx, repository.QueryExportReportJobFilter{
		JobID: params.JobID,
		Limit: constant.QueryLimitOne,
	})
	if err != nil {
		return nil, err
	}

	if len(jobs) == 0 {
		return nil, errs.New(constant.NotFound, fmt.Sprintf("job not found: %d", params.JobID))
	}

	job := jobs[0]
	var updateTime time.Time
	if job.UpdateTime != nil {
		updateTime = time.UnixMilli(*job.UpdateTime)
	}
	result := &usecase.GetExportReportJobResult{
		JobID:        job.ID,
		Status:       job.Status,
		CreationTime: time.UnixMilli(job.CreationTime),
		UpdateTime:   updateTime,
	}

	switch job.Status {
	case constant.ExportReportJobStatusSuccess:
		if job.Extra.FileName != nil {
			downloadURL, urlErr := u.s3Repository.GeneratePresignedDownloadURL(ctx, repository.GeneratePresignedDownloadURLParams{
				FileName:  *job.Extra.FileName,
				ExpiresIn: constant.PresignedURLDefaultExpiry,
			})
			if urlErr != nil {
				return nil, urlErr
			}
			result.DownloadURL = downloadURL
		}
	case constant.ExportReportJobStatusFailed:
		if job.Extra.ErrMsg != nil {
			result.ErrorMessage = *job.Extra.ErrMsg
		}
	}

	return result, nil
}
