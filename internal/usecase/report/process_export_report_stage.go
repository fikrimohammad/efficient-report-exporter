package report

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"time"

	"github.com/bytedance/sonic"
	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/model"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
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
			lastOrderSettlementTime time.Time
			hasCursor               bool
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
				HasCursor:               hasCursor,
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
				lastOrderSettlementTime = r.OrderSettlementTime
				hasCursor = true
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

		// details is reused across reports so the JSON unmarshal amortizes its
		// backing array instead of allocating a fresh slice per report.
		var details model.ReportFeeDetails

		for {
			reportData, err := reportDataStream.Read(ctx)
			if err != nil {
				if !errors.Is(err, typedpipe.ErrPipeClosed) {
					return err
				}

				return nil
			}

			details = details[:0]
			if err := sonic.Unmarshal(reportData.Details, &details); err != nil {
				return err
			}

			for _, reportDetail := range details {
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
	mainPipeline *errgroup.Group,
	reportLineDataStream typedpipe.Reader[model.ReportLine],
) (
	io.ReadCloser,
	error,
) {
	reportFileReader, reportFileWriter := nio.Pipe(buffer.New(constant.DefaultPipeBufferSize))

	mainPipeline.Go(func(ctx context.Context) error {
		defer func() { _ = reportFileWriter.Close() }()

		csvBufSize := e.dynamicConfig.Data().CSVWriteBufSize.GetWithDefault(ctx, constant.DefaultCSVWriteBufSize)
		reportFileWriterBuf := bufio.NewWriterSize(reportFileWriter, csvBufSize)
		defer func() { _ = reportFileWriterBuf.Flush() }()

		builder, err := newReportCSVBuilder(reportFileWriterBuf)
		if err != nil {
			_ = reportFileWriter.CloseWithError(err)
			return err
		}

		for {
			reportLine, err := reportLineDataStream.Read(ctx)
			if err != nil {
				if !errors.Is(err, typedpipe.ErrPipeClosed) {
					_ = reportFileWriter.CloseWithError(err)
					return err
				}

				return nil
			}

			if writeErr := builder.appendRow(reportLine); writeErr != nil {
				_ = reportFileWriter.CloseWithError(writeErr)
				return writeErr
			}
		}
	})

	return reportFileReader, nil
}

// compressReportCSVFile deflates the batch's CSV in the calling (batch worker)
// goroutine, computing the CRC32 and sizes required to write the entry via
// zip.Writer.CreateRaw. This moves the CPU-bound deflate off the single zip
// goroutine and onto the parallel batch workers.
func (e *reportExporter) compressReportCSVFile(name string, csv io.ReadCloser) (model.ReportBatchFile, error) {
	var buf bytes.Buffer
	fw, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return model.ReportBatchFile{}, err
	}

	h := crc32.NewIEEE()
	uncompressed, err := io.Copy(io.MultiWriter(fw, h), csv)
	if err != nil {
		return model.ReportBatchFile{}, err
	}
	if err := fw.Close(); err != nil {
		return model.ReportBatchFile{}, err
	}
	if err := csv.Close(); err != nil {
		return model.ReportBatchFile{}, err
	}

	return model.ReportBatchFile{
		Name:             name,
		Data:             buf.Bytes(),
		CRC32:            h.Sum32(),
		CompressedSize:   uint64(buf.Len()),
		UncompressedSize: uint64(uncompressed),
	}, nil
}

func (e *reportExporter) asyncBuildReportBatchFiles(
	mainPipeline *errgroup.Group,
	shopID int64,
	startTime, endTime time.Time,
) (
	typedpipe.Reader[model.ReportBatchFile],
	error,
) {
	batchFileWriter, batchFileReader := typedpipe.New[model.ReportBatchFile]()

	mainPipeline.Go(func(ctx context.Context) error {
		batches := e.buildReportBatches(ctx, shopID, startTime, endTime)
		workers := e.dynamicConfig.Data().MaxBatchPipelineWorkers.GetWithDefault(ctx, constant.DefaultMaxBatchPipelineWorkers)

		batchGroup := mainPipeline.SubGroup(errgroup.WithMaxConcurrency(workers))

		for _, b := range batches {
			batchGroup.Go(func(ctx context.Context) error {
				sub := batchGroup.SubGroup()

				reports, fetchErr := e.asyncFetchReports(sub, b.ShopID, b.StartTime, b.EndTime)
				if fetchErr != nil {
					return fetchErr
				}

				lines, lineErr := e.asyncBuildReportLine(sub, reports)
				if lineErr != nil {
					return lineErr
				}

				csv, csvErr := e.asyncBuildReportCSVFile(sub, lines)
				if csvErr != nil {
					return csvErr
				}

				compressed, compErr := e.compressReportCSVFile(b.EntryName(), csv)
				if compErr != nil {
					return compErr
				}

				if err := batchFileWriter.Write(ctx, compressed); err != nil {
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

			entry, entryErr := zw.CreateRaw(&zip.FileHeader{
				Name:               batchFile.Name,
				Method:             zip.Deflate,
				CRC32:              batchFile.CRC32,
				CompressedSize64:   batchFile.CompressedSize,
				UncompressedSize64: batchFile.UncompressedSize,
			})
			if entryErr != nil {
				_ = zipWriter.CloseWithError(entryErr)
				return entryErr
			}

			if _, writeErr := entry.Write(batchFile.Data); writeErr != nil {
				_ = zipWriter.CloseWithError(writeErr)
				return writeErr
			}
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
