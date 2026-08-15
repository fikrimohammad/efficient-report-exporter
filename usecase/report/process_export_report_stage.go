package report

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"

	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/go-dev-sdk/errgroup"
	"github.com/fikrimohammad/go-typedpipe/v2"
)

func buildReportFileName(jobID int64, ext string) string {
	return fmt.Sprintf("report_%d%s", jobID, ext)
}

func (e *reportExporter) buildReportBatches(ctx context.Context, shopID int64, startTime, endTime time.Time) []model.ReportBatch {
	batchSize := e.dynamicConfig.Data().MaxTimeRangePerBatch.GetWithDefault(ctx, constant.DefaultMaxTimeRangePerBatch)
	if batchSize <= 0 {
		batchSize = constant.DefaultMaxTimeRangePerBatch
	}

	var batches []model.ReportBatch
	for t := startTime; t.Before(endTime); {
		batchEnd := t.Add(batchSize)
		if batchEnd.After(endTime) {
			batchEnd = endTime
		}

		batches = append(batches, model.ReportBatch{ShopID: shopID, StartTime: t, EndTime: batchEnd})
		t = batchEnd
	}

	return batches
}

func (e *reportExporter) asyncFetchReports(
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
			lastReportID            int64
			lastOrderSettlementTime *time.Time
			limitPerPage            = e.dynamicConfig.Data().QueryLimitPerPage.GetWithDefault(ctx, constant.DefaultQueryLimitPerPage)
		)

		for {
			reports, err := e.mysqlRepository.QueryReport(ctx, repository.QueryReportFilter{
				ShopID: &shopID,
				OrderSettlementTimeRange: &repository.QueryReportTimeRange{
					StartTime: &startTime,
					EndTime:   &endTime,
				},
				Limit:                   limitPerPage,
				LastReportID:            lastReportID,
				LastOrderSettlementTime: lastOrderSettlementTime,
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
				t := r.OrderSettlementTime
				lastOrderSettlementTime = &t
			}

			if len(reports) < limitPerPage {
				break
			}
		}

		return nil
	})

	return reportsDataStreamReader, nil
}

func (e *reportExporter) asyncBuildReportLine(
	ctx context.Context,
	mainPipeline *errgroup.Group,
	reportDataStream typedpipe.Reader[model.Report],
) (
	typedpipe.Reader[model.ReportLine],
	error,
) {
	var (
		reportLineWriter, reportLineReader = typedpipe.New[model.ReportLine]()
	)

	mainPipeline.Go(func(ctx context.Context) error {
		defer reportLineWriter.Close()

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

	return reportLineReader, nil
}

func (e *reportExporter) asyncBuildReportCSVFile(
	ctx context.Context,
	mainPipeline *errgroup.Group,
	reportLineDataStream typedpipe.Reader[model.ReportLine],
) (
	io.ReadCloser,
	error,
) {
	var (
		reportFileReader, reportFileWriter = nio.Pipe(buffer.New(constant.DefaultPipeBufferSize))
		csvBufSize                         = e.dynamicConfig.Data().CSVWriteBufSize.GetWithDefault(ctx, constant.DefaultCSVWriteBufSize)
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
					_ = reportFileWriter.CloseWithError(err)
					return err
				}

				return nil
			}

			if writerError := reportFileCSVWriter.Write(reportLine.ToCSVRow()); writerError != nil {
				_ = reportFileWriter.CloseWithError(writerError)
				return writerError
			}
		}
	})

	return reportFileReader, nil
}

func (e *reportExporter) asyncBuildReportBatchFiles(
	ctx context.Context,
	mainPipeline *errgroup.Group,
	shopID int64,
	startTime, endTime time.Time,
) (
	typedpipe.Reader[model.ReportBatchFile],
	error,
) {
	var (
		batches                          = e.buildReportBatches(ctx, shopID, startTime, endTime)
		workers                          = e.dynamicConfig.Data().MaxBatchPipelineWorkers.GetWithDefault(ctx, constant.DefaultMaxBatchPipelineWorkers)
		batchFileWriter, batchFileReader = typedpipe.New[model.ReportBatchFile]()
	)

	mainPipeline.Go(func(ctx context.Context) error {
		batchGroup := mainPipeline.SubGroup(errgroup.WithMaxConcurrency(workers))

		for _, b := range batches {
			batchGroup.Go(func(ctx context.Context) error {
				sub := batchGroup.SubGroup()

				reports, fetchErr := e.asyncFetchReports(sub, b.ShopID, b.StartTime, b.EndTime)
				if fetchErr != nil {
					return fetchErr
				}

				lines, lineErr := e.asyncBuildReportLine(ctx, sub, reports)
				if lineErr != nil {
					return lineErr
				}

				csv, csvErr := e.asyncBuildReportCSVFile(ctx, sub, lines)
				if csvErr != nil {
					return csvErr
				}

				if err := batchFileWriter.Write(ctx, model.ReportBatchFile{Name: b.EntryName(), Reader: csv}); err != nil {
					return err
				}

				return sub.Wait()
			})
		}

		if err := batchGroup.Wait(); err != nil {
			batchFileWriter.CloseWithError(err)
			return err
		}

		batchFileWriter.Close()
		return nil
	})

	return batchFileReader, nil
}

func (e *reportExporter) asyncZipReportBatchFiles(mainPipeline *errgroup.Group, in typedpipe.Reader[model.ReportBatchFile]) (io.ReadCloser, error) {
	var (
		zipReader, zipWriter = nio.Pipe(buffer.New(constant.DefaultPipeBufferSize))
	)

	mainPipeline.Go(func(ctx context.Context) error {
		zw := zip.NewWriter(zipWriter)

		for {
			batchFile, err := in.Read(ctx)
			if err != nil {
				if errors.Is(err, typedpipe.ErrPipeClosed) {
					if closeErr := zw.Close(); closeErr != nil {
						_ = zipWriter.CloseWithError(closeErr)
						return closeErr
					}

					_ = zipWriter.Close()
					return nil
				}

				_ = zipWriter.CloseWithError(err)
				return err
			}

			entry, entryErr := zw.CreateHeader(&zip.FileHeader{Name: batchFile.Name, Method: zip.Deflate})
			if entryErr != nil {
				_ = zipWriter.CloseWithError(entryErr)
				return entryErr
			}

			if _, copyErr := io.Copy(entry, batchFile.Reader); copyErr != nil {
				_ = zipWriter.CloseWithError(copyErr)
				return copyErr
			}

			_ = batchFile.Reader.Close()
		}
	})

	return zipReader, nil
}

func (e *reportExporter) asyncUploadReportFile(mainPipeline *errgroup.Group, fileData io.ReadCloser, fileName string) error {
	mainPipeline.Go(func(ctx context.Context) error {
		defer func() { _ = fileData.Close() }()

		return e.s3Repository.UploadReportFile(ctx, repository.UploadReportFileParams{
			FileName: fileName,
			FileData: fileData,
		})
	})

	return nil
}
