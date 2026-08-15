package report

import (
	"context"
	"fmt"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/config"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-dev-sdk/confloader"
	"github.com/fikrimohammad/go-dev-sdk/errgroup"
	"github.com/fikrimohammad/go-dev-sdk/errs"
	"github.com/fikrimohammad/go-dev-sdk/observability/logs"
)

func (u *useCase) ProcessExportReport(ctx context.Context, params usecase.ProcessExportReportParams) error {
	e := reportExporter{
		mysqlRepository: u.mysqlRepository,
		mqRepository:    u.mqRepository,
		redisRepository: u.redisRepository,
		s3Repository:    u.s3Repository,
		dynamicConfig:   u.dynamicConfig,
	}

	return e.Export(ctx, params)
}

type reportExporter struct {
	mysqlRepository repository.MySQL
	mqRepository    repository.MQ
	redisRepository repository.Redis
	s3Repository    repository.S3
	dynamicConfig   *confloader.Loader[config.DynamicConfig]
}

func (e *reportExporter) Export(ctx context.Context, params usecase.ProcessExportReportParams) error {
	lockToken, err := e.lockExportReportJob(ctx, params)
	if err != nil {
		return err
	}

	defer func() {
		_ = e.unlockExportReportJob(ctx, params, lockToken)
	}()

	job, queryErr := e.queryExportReportJob(ctx, params)
	if queryErr != nil {
		return queryErr
	}

	if job.Status == constant.ExportReportJobStatusSuccess {
		return nil
	}

	err = e.runExportReportPipeline(ctx, job.ShopID, time.UnixMilli(job.StartTime), time.UnixMilli(job.EndTime), params.JobID)
	if err != nil {
		return err
	}

	return nil
}

func (e *reportExporter) lockExportReportJob(ctx context.Context, params usecase.ProcessExportReportParams) (string, error) {
	lockTTL := e.dynamicConfig.Data().ProcessLockTTL.GetWithDefault(ctx, constant.DefaultProcessLockTTL)
	token, err := e.redisRepository.LockExportReportProcess(ctx, repository.LockExportReportProcess{
		JobID: params.JobID,
		TTL:   lockTTL,
	})
	if err != nil {
		return "", err
	}

	return token, nil
}

func (e *reportExporter) unlockExportReportJob(ctx context.Context, params usecase.ProcessExportReportParams, token string) error {
	err := e.redisRepository.UnlockExportReportProcess(ctx, repository.UnlockExportReportProcess{
		JobID: params.JobID,
		Token: token,
	})
	if err != nil {
		return err
	}

	return nil
}

func (e *reportExporter) queryExportReportJob(ctx context.Context, params usecase.ProcessExportReportParams) (*model.ExportReportJob, error) {
	jobs, err := e.mysqlRepository.QueryExportReportJob(ctx, repository.QueryExportReportJobFilter{
		JobID: params.JobID,
		Limit: constant.QueryLimitOne,
	})
	if err != nil {
		return nil, err
	}

	if len(jobs) == 0 {
		return nil, errs.New(apperrors.NotFound, fmt.Sprintf("job not found: %d", params.JobID))
	}

	return jobs[0], nil
}

func (e *reportExporter) runExportReportPipeline(ctx context.Context, shopID int64, startTime, endTime time.Time, jobID int64) error {
	var (
		err          error
		mainPipeline = errgroup.New(ctx)
	)

	defer func() {
		if err != nil {
			errMsg := err.Error()
			if updateErr := e.mysqlRepository.UpdateExportReportJob(ctx, repository.UpdateExportReportJobParams{
				JobID:  jobID,
				Status: constant.ExportReportJobStatusFailed,
				Extra: model.ExportReportJobExtra{
					ErrMsg: &errMsg,
				},
			}); updateErr != nil {
				err = updateErr
			}

			return
		}
	}()

	total, countErr := e.mysqlRepository.CountReport(ctx, repository.QueryReportFilter{
		ShopID: &shopID,
		OrderSettlementTimeRange: &repository.QueryReportTimeRange{
			StartTime: &startTime,
			EndTime:   &endTime,
		},
	})
	if countErr != nil {
		err = countErr
		return err
	}

	maxSingleFileRows := e.dynamicConfig.Data().MaxSingleFileRows.GetWithDefault(ctx, constant.DefaultMaxSingleFileRows)

	var reportFileName string
	if total <= int64(maxSingleFileRows) {
		reportFileName, err = e.runSinglePipeline(ctx, mainPipeline, shopID, startTime, endTime, jobID)
	} else {
		reportFileName, err = e.runBatchedPipeline(ctx, mainPipeline, shopID, startTime, endTime, jobID)
	}
	if err != nil {
		return err
	}

	if pipeErr := mainPipeline.Wait(); pipeErr != nil {
		err = pipeErr
		return err
	}

	updateErr := e.mysqlRepository.UpdateExportReportJob(ctx, repository.UpdateExportReportJobParams{
		JobID:  jobID,
		Status: constant.ExportReportJobStatusSuccess,
		Extra: model.ExportReportJobExtra{
			FileName: &reportFileName,
		},
	})
	if updateErr != nil {
		err = updateErr
		return err
	}

	if pubErr := e.mqRepository.PublishExportReportDoneMsg(ctx, model.ExportReportDoneMessage{
		JobID: jobID,
	}); pubErr != nil {
		logs.Warn(ctx, "failed to publish export report done message", "job.id", jobID, "error", pubErr)
	}

	return nil
}

func (e *reportExporter) runSinglePipeline(ctx context.Context, mainPipeline *errgroup.Group, shopID int64, startTime, endTime time.Time, jobID int64) (string, error) {
	reports, fetchErr := e.asyncFetchReports(mainPipeline, shopID, startTime, endTime)
	if fetchErr != nil {
		return "", fetchErr
	}

	lines, lineErr := e.asyncBuildReportLine(ctx, mainPipeline, reports)
	if lineErr != nil {
		return "", lineErr
	}

	csv, csvErr := e.asyncBuildReportCSVFile(ctx, mainPipeline, lines)
	if csvErr != nil {
		return "", csvErr
	}

	fileName := buildReportFileName(jobID, constant.ReportCSVExtension)
	if uploadErr := e.asyncUploadReportFile(mainPipeline, csv, fileName); uploadErr != nil {
		return "", uploadErr
	}

	return fileName, nil
}

func (e *reportExporter) runBatchedPipeline(ctx context.Context, mainPipeline *errgroup.Group, shopID int64, startTime, endTime time.Time, jobID int64) (string, error) {
	batchFiles, batchErr := e.asyncBuildReportBatchFiles(ctx, mainPipeline, shopID, startTime, endTime)
	if batchErr != nil {
		return "", batchErr
	}

	zipReader, zipErr := e.asyncZipReportBatchFiles(mainPipeline, batchFiles)
	if zipErr != nil {
		return "", zipErr
	}

	fileName := buildReportFileName(jobID, constant.ReportZipExtension)
	if uploadErr := e.asyncUploadReportFile(mainPipeline, zipReader, fileName); uploadErr != nil {
		return "", uploadErr
	}

	return fileName, nil
}
