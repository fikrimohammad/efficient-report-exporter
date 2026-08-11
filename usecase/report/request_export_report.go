package report

import (
	"context"

	"github.com/fikrimohammad/go-dev-sdk/errs"
	"github.com/fikrimohammad/go-dev-sdk/observability/logs"
	"github.com/fikrimohammad/go-dev-sdk/observability/tracer"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
)

// RequestExportReport is a use case that handles the request to export a report.
func (u *useCase) RequestExportReport(ctx context.Context, params usecase.RequestExportReportParams) (*usecase.RequestExportReportResult, error) {
	rr := &reportRequester{
		mySQLRepository: u.mySQLRepository,
		mqRepository:    u.mqRepository,
		redisRepository: u.redisRepository,
	}

	return rr.Request(ctx, params)
}

type reportRequester struct {
	mySQLRepository repository.MySQL
	mqRepository    repository.MQ
	redisRepository repository.Redis
}

func (rr *reportRequester) Request(ctx context.Context, params usecase.RequestExportReportParams) (result *usecase.RequestExportReportResult, err error) {
	if err = params.Validate(); err != nil {
		return nil, err
	}

	if err = rr.lockRequest(ctx, params); err != nil {
		return nil, err
	}
	defer func() { _ = rr.unlockRequest(ctx, params) }()

	job, queryErr := rr.queryExportReportJob(ctx, params)
	if queryErr != nil {
		err = queryErr
		return nil, err
	}

	if job != nil {
		if job.Status == constant.ExportReportJobStatusSuccess || job.Status == constant.ExportReportJobStatusProcessing {
			logs.Info(ctx, "export job reused", jobLogAttrs(params, job.ID, tracer.TraceIDFrom(ctx))...)
			return &usecase.RequestExportReportResult{JobID: job.ID}, nil
		}
	} else {
		if checkErr := rr.checkReportDataExistence(ctx, params); checkErr != nil {
			err = checkErr
			return nil, err
		}

		newJob, initErr := rr.initExportReportJob(ctx, params)
		if initErr != nil {
			err = initErr
			return nil, err
		}

		job = newJob
		logs.Info(ctx, "export job created", jobLogAttrs(params, job.ID, tracer.TraceIDFrom(ctx))...)
	}

	if startErr := rr.startExportReportJob(ctx, job); startErr != nil {
		err = startErr
		return nil, err
	}

	return &usecase.RequestExportReportResult{JobID: job.ID}, nil
}

func (rr *reportRequester) lockRequest(ctx context.Context, params usecase.RequestExportReportParams) error {
	return rr.redisRepository.LockExportReportRequest(ctx, repository.LockExportReportRequest{
		RequestID: params.RequestID,
		TTL:       constant.DefaultRequestLockTTL,
	})
}

func (rr *reportRequester) unlockRequest(ctx context.Context, params usecase.RequestExportReportParams) error {
	return rr.redisRepository.UnlockExportReportRequest(ctx, repository.UnlockExportReportRequest{
		RequestID: params.RequestID,
	})
}

func (rr *reportRequester) queryExportReportJob(ctx context.Context, params usecase.RequestExportReportParams) (*model.ExportReportJob, error) {
	jobs, err := rr.mySQLRepository.QueryExportReportJob(ctx, repository.QueryExportReportJobFilter{
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

func (rr *reportRequester) checkReportDataExistence(ctx context.Context, params usecase.RequestExportReportParams) error {
	reports, err := rr.mySQLRepository.QueryReport(ctx, repository.QueryReportFilter{
		ShopID: &params.ShopID,
		OrderSettlementTimeRange: &repository.QueryReportTimeRange{
			StartTime: &params.StartTime,
			EndTime:   &params.EndTime,
		},
		Limit: constant.SingleRowQueryLimit,
	})
	if err != nil {
		return err
	}

	if len(reports) == 0 {
		return errs.New(errs.NotFound, "report data not found")
	}

	return nil
}

func (rr *reportRequester) initExportReportJob(ctx context.Context, params usecase.RequestExportReportParams) (*model.ExportReportJob, error) {
	job, err := rr.mySQLRepository.InsertExportReportJob(ctx, repository.InsertExportReportJobParams{
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

func (rr *reportRequester) startExportReportJob(ctx context.Context, job *model.ExportReportJob) error {
	err := rr.mqRepository.PublishExportReportProcessMsg(
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
