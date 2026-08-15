package report

import (
	"context"

	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/config"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-dev-sdk/confloader"
	"github.com/fikrimohammad/go-dev-sdk/errs"
	"github.com/fikrimohammad/go-dev-sdk/observability/logs"
	"github.com/fikrimohammad/go-dev-sdk/observability/tracer"
)

// RequestExportReport is a use case that handles the request to export a report.
func (u *useCase) RequestExportReport(ctx context.Context, params usecase.RequestExportReportParams) (*usecase.RequestExportReportResult, error) {
	r := &reportRequester{
		mysqlRepository: u.mysqlRepository,
		mqRepository:    u.mqRepository,
		redisRepository: u.redisRepository,
		dynamicConfig:   u.dynamicConfig,
	}

	return r.Request(ctx, params)
}

type reportRequester struct {
	mysqlRepository repository.MySQL
	mqRepository    repository.MQ
	redisRepository repository.Redis
	dynamicConfig   *confloader.Loader[config.DynamicConfig]
}

func (r *reportRequester) Request(ctx context.Context, params usecase.RequestExportReportParams) (result *usecase.RequestExportReportResult, err error) {
	if err = params.Validate(); err != nil {
		return nil, err
	}

	var lockToken string
	if lockToken, err = r.lockRequest(ctx, params); err != nil {
		return nil, err
	}
	defer func() { _ = r.unlockRequest(ctx, params, lockToken) }()

	job, queryErr := r.queryExportReportJob(ctx, params)
	if queryErr != nil {
		err = queryErr
		return nil, err
	}

	if job != nil {
		if job.Status == constant.ExportReportJobStatusSuccess || job.Status == constant.ExportReportJobStatusProcessing {
			logs.Info(ctx, "export job reused", jobLogAttrs(params, job.ID, tracer.TraceIDFrom(ctx))...)
			return &usecase.RequestExportReportResult{JobID: job.ID}, nil
		}

		// A previously failed job is retried in place: reset it to processing
		// before re-enqueueing so its status reflects the pending retry.
		if resetErr := r.resetExportReportJob(ctx, job.ID); resetErr != nil {
			err = resetErr
			return nil, err
		}
		logs.Info(ctx, "export job retried", jobLogAttrs(params, job.ID, tracer.TraceIDFrom(ctx))...)
	} else {
		if checkErr := r.checkReportDataExistence(ctx, params); checkErr != nil {
			err = checkErr
			return nil, err
		}

		newJob, initErr := r.createExportReportJob(ctx, params)
		if initErr != nil {
			err = initErr
			return nil, err
		}

		job = newJob
		logs.Info(ctx, "export job created", jobLogAttrs(params, job.ID, tracer.TraceIDFrom(ctx))...)
	}

	if startErr := r.publishExportReportProcessMsg(ctx, job); startErr != nil {
		err = startErr
		return nil, err
	}

	return &usecase.RequestExportReportResult{JobID: job.ID}, nil
}

func (r *reportRequester) lockRequest(ctx context.Context, params usecase.RequestExportReportParams) (string, error) {
	lockTTL := r.dynamicConfig.Data().RequestLockTTL.GetWithDefault(ctx, constant.DefaultRequestLockTTL)
	return r.redisRepository.LockExportReportRequest(ctx, repository.LockExportReportRequest{
		RequestID: params.RequestID,
		TTL:       lockTTL,
	})
}

func (r *reportRequester) unlockRequest(ctx context.Context, params usecase.RequestExportReportParams, token string) error {
	return r.redisRepository.UnlockExportReportRequest(ctx, repository.UnlockExportReportRequest{
		RequestID: params.RequestID,
		Token:     token,
	})
}

func (r *reportRequester) queryExportReportJob(ctx context.Context, params usecase.RequestExportReportParams) (*model.ExportReportJob, error) {
	jobs, err := r.mysqlRepository.QueryExportReportJob(ctx, repository.QueryExportReportJobFilter{
		RequestID: params.RequestID,
	})
	if err != nil {
		return nil, err
	}

	if len(jobs) == 0 {
		return nil, nil
	}

	return jobs[0], nil
}

func (r *reportRequester) checkReportDataExistence(ctx context.Context, params usecase.RequestExportReportParams) error {
	reports, err := r.mysqlRepository.QueryReport(ctx, repository.QueryReportFilter{
		ShopID: &params.ShopID,
		OrderSettlementTimeRange: &repository.QueryReportTimeRange{
			StartTime: &params.StartTime,
			EndTime:   &params.EndTime,
		},
		Limit: constant.QueryLimitOne,
	})
	if err != nil {
		return err
	}

	if len(reports) == 0 {
		return errs.New(apperrors.NotFound, "report data not found")
	}

	return nil
}

func (r *reportRequester) createExportReportJob(ctx context.Context, params usecase.RequestExportReportParams) (*model.ExportReportJob, error) {
	job, err := r.mysqlRepository.InsertExportReportJob(ctx, repository.InsertExportReportJobParams{
		ShopID:    params.ShopID,
		RequestID: params.RequestID,
		StartTime: params.StartTime.UnixMilli(),
		EndTime:   params.EndTime.UnixMilli(),
		Status:    constant.ExportReportJobStatusProcessing,
		Extra:     model.ExportReportJobExtra{},
	})
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (r *reportRequester) resetExportReportJob(ctx context.Context, jobID int64) error {
	return r.mysqlRepository.UpdateExportReportJob(ctx, repository.UpdateExportReportJobParams{
		JobID:  jobID,
		Status: constant.ExportReportJobStatusProcessing,
		Extra:  model.ExportReportJobExtra{},
	})
}

func (r *reportRequester) publishExportReportProcessMsg(ctx context.Context, job *model.ExportReportJob) error {
	err := r.mqRepository.PublishExportReportProcessMsg(
		ctx,
		model.ExportReportProcessMessage{
			JobID: job.ID,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func jobLogAttrs(params usecase.RequestExportReportParams, jobID int64, traceID string) []any {
	attrs := []any{
		"job.id", jobID,
		"shop.id", params.ShopID,
		"request.id", params.RequestID,
	}
	if traceID != "" {
		attrs = append(attrs, "trace.id", traceID)
	}
	return attrs
}
