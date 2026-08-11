package report

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/fikrimohammad/go-dev-sdk/confloader"
	"github.com/fikrimohammad/go-dev-sdk/errgroup"
	"github.com/fikrimohammad/go-dev-sdk/errs"
	"github.com/fikrimohammad/efficient-report-exporter/config"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-typedpipe/v2"
	"github.com/google/uuid"
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

	reportDataStream, fetchReportErr := re.asyncFetchReports(mainPipeline, shopID, startTime, endTime)
	if fetchReportErr != nil {
		err = fetchReportErr
		return err
	}

	reportLineDataStream, buildReportErr := re.asyncBuildReportLine(ctx, mainPipeline, reportDataStream)
	if buildReportErr != nil {
		err = buildReportErr
		return err
	}

	reportCSVFileDataStream, buildCSVFileErr := re.asyncBuildReportCSVFile(ctx, mainPipeline, reportLineDataStream)
	if buildCSVFileErr != nil {
		err = buildCSVFileErr
		return err
	}

	reportFileName, uploadErr := re.asyncUploadReportFile(mainPipeline, reportCSVFileDataStream)
	if uploadErr != nil {
		err = uploadErr
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

func (re *reportExporter) asyncFetchReports(
	mainPipeline *errgroup.Group,
	shopID int64,
	startTime, endTime time.Time,
) (
	typedpipe.Reader[model.Report],
	error,
) {
	var (
		reportsDataStreamWriter, reportsDataStreamReader = typedpipe.New[model.Report]()
	)

	mainPipeline.Go(func(ctx context.Context) error {
		defer reportsDataStreamWriter.Close()

		var (
			lastReportID int64
			limitPerPage = re.dynamicConfig.Data().QueryLimitPerPage.GetWithDefault(ctx, constant.DefaultQueryLimitPerPage)
		)

		for {
			reports, err := re.mysqlRepository.QueryReport(ctx, repository.QueryReportFilter{
				ShopID: &shopID,
				OrderSettlementTimeRange: &repository.QueryReportTimeRange{
					StartTime: &startTime,
					EndTime:   &endTime,
				},
				Limit:        limitPerPage,
				LastReportID: lastReportID,
			})
			if err != nil {
				reportsDataStreamWriter.CloseWithError(err)
				return err
			}

			if len(reports) == 0 {
				return nil
			}

			for _, r := range reports {
				writerErr := reportsDataStreamWriter.Write(ctx, *r)
				if writerErr != nil {
					reportsDataStreamWriter.CloseWithError(writerErr)
					return writerErr
				}
				lastReportID = r.ID
			}

			if len(reports) < limitPerPage {
				break
			}
		}

		return nil
	})

	return reportsDataStreamReader, nil
}

func (re *reportExporter) asyncBuildReportLine(
	ctx context.Context,
	mainPipeline *errgroup.Group,
	reportDataStream typedpipe.Reader[model.Report],
) (
	typedpipe.Reader[model.ReportLine],
	error,
) {
	var (
		reportLineWriter, reportLineReader = typedpipe.New[model.ReportLine]()
		reportLineWorkerPoolCount          = re.dynamicConfig.Data().ReportLineWorkers.GetWithDefault(ctx, constant.DefaultReportLineWorkers)
		reportLinePipeline                 = mainPipeline.SubGroup(errgroup.WithMaxConcurrency(reportLineWorkerPoolCount))
	)

	for i := 0; i < reportLineWorkerPoolCount; i++ {
		reportLinePipeline.Go(func(ctx context.Context) error {
			for {
				reportData, err := reportDataStream.Read(ctx)
				if err != nil {
					if !errors.Is(err, typedpipe.ErrPipeClosed) {
						return err
					}

					return nil
				}

				for _, reportDetail := range reportData.Details {
					writerError := reportLineWriter.Write(ctx, model.ReportLine{
						ShopID:              reportData.ShopID,
						OrderID:             reportData.OrderID,
						OrderCreationTime:   reportData.OrderCreationTime,
						OrderPaymentTime:    reportData.OrderPaymentTime,
						OrderSettlementTime: reportData.OrderSettlementTime,
						FeeID:               reportData.FeeID,
						ReportFeeDetail:     reportDetail,
					})
					if writerError != nil {
						return writerError
					}
				}

			}
		})
	}

	mainPipeline.Go(func(ctx context.Context) error {
		if err := reportLinePipeline.Wait(); err != nil {
			reportLineWriter.CloseWithError(err)
			return err
		}

		reportLineWriter.Close()
		return nil
	})

	return reportLineReader, nil
}

func (re *reportExporter) asyncBuildReportCSVFile(
	ctx context.Context,
	mainPipeline *errgroup.Group,
	reportLineDataStream typedpipe.Reader[model.ReportLine],
) (
	io.ReadCloser,
	error,
) {
	var (
		reportFileReader, reportFileWriter = io.Pipe()
		csvBufSize                         = re.dynamicConfig.Data().CSVWriteBufSize.GetWithDefault(ctx, constant.DefaultCSVWriteBufSize)
		reportFileCSVWriter                = csv.NewWriter(bufio.NewWriterSize(reportFileWriter, csvBufSize))
	)

	if err := reportFileCSVWriter.Write(constant.ReportFileCSVHeaders); err != nil {
		if closeErr := reportFileWriter.CloseWithError(err); closeErr != nil {
			return nil, closeErr
		}

		return nil, err
	}

	mainPipeline.Go(func(ctx context.Context) error {
		defer func() {
			reportFileCSVWriter.Flush()
			_ = reportFileWriter.Close()
		}()

		for {
			reportLine, err := reportLineDataStream.Read(ctx)
			if err != nil {
				if !errors.Is(err, typedpipe.ErrPipeClosed) {
					reportFileWriter.CloseWithError(err)
					return err
				}

				return nil
			}

			if writerError := reportFileCSVWriter.Write(reportLine.ToCSVRow()); writerError != nil {
				reportFileWriter.CloseWithError(writerError)
				return writerError
			}
		}
	})

	return reportFileReader, nil
}

func (re *reportExporter) asyncUploadReportFile(mainPipeline *errgroup.Group, reportCSVFileDataStream io.ReadCloser) (string, error) {
	fileID, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	fileName := fmt.Sprintf("%s.csv", fileID.String())

	mainPipeline.Go(func(ctx context.Context) error {
		defer func() { _ = reportCSVFileDataStream.Close() }()

		return re.s3Repository.UploadReportFile(ctx, repository.UploadReportFileParams{
			FileName: fileName,
			FileData: reportCSVFileDataStream,
		})
	})

	return fileName, nil
}
