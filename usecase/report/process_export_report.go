package report

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/config"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-dev-sdk/confloader"
	"github.com/fikrimohammad/go-dev-sdk/errgroup"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

func (u *useCase) ProcessExportReport(ctx context.Context, params usecase.ProcessExportReportParams) error {
	re := reportExporter{
		mysqlRepository: u.mySQLRepository,
		mqRepository:    u.mqRepository,
		redisRepository: u.redisRepository,
		s3Repository:    u.s3Repository,
		dynamicConfig:   u.dynamicConfig,
	}

	return re.Export(ctx, params)
}

type reportExporter struct {
	mysqlRepository repository.MySQL
	mqRepository    repository.MQ
	redisRepository repository.Redis
	s3Repository    repository.S3
	dynamicConfig   *confloader.Loader[config.DynamicConfig]
}

func (re *reportExporter) Export(ctx context.Context, params usecase.ProcessExportReportParams) error {
	var (
		err          error
		lockAcquired bool
	)

	if err := re.lockExportReportJob(ctx, params); err != nil {
		return err
	}
	lockAcquired = true

	defer func() {
		if lockAcquired {
			_ = re.unlockExportReportJob(ctx, params)
		}
	}()

	job, queryErr := re.queryExportReportJob(ctx, params)
	if queryErr != nil {
		return queryErr
	}

	if job.Status == constant.ExportReportJobStatusSuccess {
		return nil
	}

	err = re.runExportReportPipeline(ctx, job.ShopID, time.UnixMilli(job.StartTime), time.UnixMilli(job.EndTime), params.JobID)
	if err != nil {
		return err
	}

	return nil
}

func (re *reportExporter) lockExportReportJob(ctx context.Context, params usecase.ProcessExportReportParams) error {
	lockTTL := re.dynamicConfig.Data().ProcessLockTTL.GetWithDefault(ctx, constant.DefaultProcessLockTTL)
	err := re.redisRepository.LockExportReportProcess(ctx, repository.LockExportReportProcess{
		JobID: params.JobID,
		TTL:   lockTTL,
	})
	if err != nil {
		return err
	}

	return nil
}

func (re *reportExporter) unlockExportReportJob(ctx context.Context, params usecase.ProcessExportReportParams) error {
	err := re.redisRepository.UnlockExportReportProcess(ctx, repository.UnlockExportReportProcess{
		JobID: params.JobID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (re *reportExporter) queryExportReportJob(ctx context.Context, params usecase.ProcessExportReportParams) (*model.ExportReportJob, error) {
	jobs, err := re.mysqlRepository.QueryExportReportJob(ctx, repository.QueryExportReportJobFilter{
		JobID: params.JobID,
		Limit: constant.SingleRowQueryLimit,
	})
	if err != nil {
		return nil, err
	}

	if len(jobs) == 0 {
		return nil, errs.New(errs.NotFound, fmt.Sprintf("job not found: %d", params.JobID))
	}

	return jobs[0], nil
}

func (re *reportExporter) runExportReportPipeline(ctx context.Context, shopID int64, startTime, endTime time.Time, jobID int64) error {
	var (
		err          error
		mainPipeline = errgroup.New(ctx)
	)

	defer func() {
		if err != nil {
			errMsg := err.Error()
			if updateErr := re.mysqlRepository.UpdateExportReportJob(ctx, repository.UpdateExportReportJobParams{
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

	total, countErr := re.mysqlRepository.CountReport(ctx, repository.QueryReportFilter{
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

	maxSingleFileRows := re.dynamicConfig.Data().MaxSingleFileRows.GetWithDefault(ctx, constant.DefaultMaxSingleFileRows)

	var reportFileName string
	if total <= int64(maxSingleFileRows) {
		reportFileName, err = re.runSinglePipeline(ctx, mainPipeline, shopID, startTime, endTime, jobID)
	} else {
		reportFileName, err = re.runBatchedPipeline(ctx, mainPipeline, shopID, startTime, endTime, jobID)
	}
	if err != nil {
		return err
	}

	if pipeErr := mainPipeline.Wait(); pipeErr != nil {
		err = pipeErr
		return err
	}

	updateErr := re.mysqlRepository.UpdateExportReportJob(ctx, repository.UpdateExportReportJobParams{
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

	if pubErr := re.mqRepository.PublishExportReportDoneMsg(ctx, model.ExportReportDoneMessage{
		JobID: jobID,
	}); pubErr != nil {
		log.Printf("failed to publish export report done message for job %d: %v", jobID, pubErr)
	}

	return nil
}

func (re *reportExporter) runSinglePipeline(ctx context.Context, mainPipeline *errgroup.Group, shopID int64, startTime, endTime time.Time, jobID int64) (string, error) {
	reports, fetchErr := re.asyncFetchReports(mainPipeline, shopID, startTime, endTime)
	if fetchErr != nil {
		return "", fetchErr
	}

	lines, lineErr := re.asyncBuildReportLine(ctx, mainPipeline, reports)
	if lineErr != nil {
		return "", lineErr
	}

	csv, csvErr := re.asyncBuildReportCSVFile(ctx, mainPipeline, lines)
	if csvErr != nil {
		return "", csvErr
	}

	fileName := buildReportFileName(jobID, constant.ReportCSVExtension)
	if uploadErr := re.asyncUploadReportFile(mainPipeline, csv, fileName); uploadErr != nil {
		return "", uploadErr
	}

	return fileName, nil
}

func (re *reportExporter) runBatchedPipeline(ctx context.Context, mainPipeline *errgroup.Group, shopID int64, startTime, endTime time.Time, jobID int64) (string, error) {
	batchFiles, batchErr := re.asyncBuildReportBatchFiles(ctx, mainPipeline, shopID, startTime, endTime)
	if batchErr != nil {
		return "", batchErr
	}

	zipReader, zipErr := re.asyncZipReportBatchFiles(mainPipeline, batchFiles)
	if zipErr != nil {
		return "", zipErr
	}

	fileName := buildReportFileName(jobID, constant.ReportZipExtension)
	if uploadErr := re.asyncUploadReportFile(mainPipeline, zipReader, fileName); uploadErr != nil {
		return "", uploadErr
	}

	return fileName, nil
}
