package report

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-typedpipe"
	"golang.org/x/sync/errgroup"
)

func (u *useCase) ExportReport(ctx context.Context, params usecase.ExportReportParams) (*usecase.ExportReportResult, error) {
	rg := reportExporter{
		reportMySQLRepository: u.reportMySQLRepository,
	}

	return rg.Export(ctx, params)
}

type reportExporter struct {
	reportMySQLRepository repository.ReportMySQL
}

func (rg *reportExporter) Export(ctx context.Context, params usecase.ExportReportParams) (*usecase.ExportReportResult, error) {
	if err := rg.validateParams(params); err != nil {
		return nil, err
	}

	errGroup, errGroupCtx := errgroup.WithContext(ctx)
	reportDataStream, err := rg.asyncFetchReports(errGroupCtx, params)
	if err != nil {
		return nil, err
	}

	reportLineDataStream, err := rg.asyncBuildReportLine(errGroupCtx, errGroup, reportDataStream)
	if err != nil {
		return nil, err
	}

	reportCSVFileDataStream, err := rg.asyncBuildReportCSVFile(errGroupCtx, errGroup, reportLineDataStream)
	if err != nil {
		return nil, err
	}

	result := &usecase.ExportReportResult{
		FileName: fmt.Sprintf(
			"%s_%s_%s.csv",
			strconv.FormatInt(params.ShopID, 10),
			params.StartTime.Format(model.ReportNameTimeFormat),
			params.EndTime.Format(model.ReportNameTimeFormat),
		),
		File: reportCSVFileDataStream,
	}

	return result, nil
}

func (rg *reportExporter) validateParams(params usecase.ExportReportParams) error {
	if params.ShopID == 0 {
		return errors.New("shop_id is required")
	}

	if params.StartTime.IsZero() {
		return errors.New("start_time is required")
	}

	if params.EndTime.IsZero() {
		return errors.New("end_time is required")
	}

	if params.StartTime.After(params.EndTime) {
		return errors.New("start time is after end time")
	}

	if params.EndTime.Sub(params.StartTime) > 365*24*time.Hour {
		return errors.New("time range exceeds duration limit (limit = 1 year)")
	}

	return nil
}

func (rg *reportExporter) asyncFetchReports(
	reportExporterErrGroupCtx context.Context,
	params usecase.ExportReportParams,
) (
	typedpipe.Reader[model.Report],
	error,
) {
	return rg.reportMySQLRepository.AsyncQueryReport(reportExporterErrGroupCtx, repository.QueryReportFilter{
		ShopID: &params.ShopID,
		OrderSettlementTimeRange: &repository.QueryReportTimeRange{
			StartTime: &params.StartTime,
			EndTime:   &params.EndTime,
		},
	})
}

func (rg *reportExporter) asyncBuildReportLine(
	reportExporterErrGroupCtx context.Context,
	reportExporterErrGroup *errgroup.Group,
	reportDataStream typedpipe.Reader[model.Report],
) (
	typedpipe.Reader[model.ReportLine],
	error,
) {
	var (
		reportLineWriter, reportLineReader, _     = typedpipe.New[model.ReportLine]()
		reportLineWorkerEg, reportLineWorkerEgCtx = errgroup.WithContext(reportExporterErrGroupCtx)
		reportLineWorkerPoolCount                 = 32
	)

	for i := 0; i < reportLineWorkerPoolCount; i++ {
		reportLineWorkerEg.Go(func() error {
			defer func() {
				if r := recover(); r != nil {
					reportLineWriter.CloseWithError(fmt.Errorf("%v", r))
				}
			}()

			for {
				reportData, err := reportDataStream.Read(reportLineWorkerEgCtx)
				if err != nil {
					if !errors.Is(err, typedpipe.ErrPipeClosed) {
						return err
					}

					return nil
				}

				for _, reportDetail := range reportData.Details {
					writerError := reportLineWriter.Write(reportLineWorkerEgCtx, model.ReportLine{
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

	reportExporterErrGroup.Go(func() error {
		if err := reportLineWorkerEg.Wait(); err != nil {
			reportLineWriter.CloseWithError(err)
			return err
		}

		reportLineWriter.Close()
		return nil
	})

	return reportLineReader, nil
}

func (rg *reportExporter) asyncBuildReportCSVFile(
	reportExporterErrGroupCtx context.Context,
	reportExporterErrGroup *errgroup.Group,
	reportLineDataStream typedpipe.Reader[model.ReportLine],
) (
	io.ReadCloser,
	error,
) {
	var (
		reportFileReader, reportFileWriter = io.Pipe()
		reportFileCSVWriter                = csv.NewWriter(bufio.NewWriterSize(reportFileWriter, 1024*1024))
		reportCSVEg, reportCSVEgCtx        = errgroup.WithContext(reportExporterErrGroupCtx)
	)

	reportCSVEg.Go(func() error {
		defer func() {
			if r := recover(); r != nil {
				reportFileWriter.CloseWithError(fmt.Errorf("%v", r))
			}
		}()

		if err := reportFileCSVWriter.Write([]string{
			"Shop ID",
			"Fee ID",
			"Order ID",
			"Order Creation Time",
			"Order Payment Time",
			"Order Settlement Time",
			"Order Detail ID",
			"Product ID",
			"Category ID",
			"Product Price Amount",
			"Promo Amount",
			"Fee Base Amount",
			"Fee Final Amount",
		}); err != nil {
			reportFileWriter.CloseWithError(err)
			return err
		}

		for {
			reportLine, err := reportLineDataStream.Read(reportCSVEgCtx)
			if err != nil {
				if !errors.Is(err, typedpipe.ErrPipeClosed) {
					reportFileWriter.CloseWithError(err)
					return err
				}

				return nil
			}

			if writerError := reportFileCSVWriter.Write([]string{
				strconv.FormatInt(reportLine.ShopID, 10),
				strconv.FormatInt(reportLine.FeeID, 10),
				strconv.FormatInt(reportLine.OrderID, 10),
				reportLine.OrderCreationTime.Format(model.ReportLineTimeFormat),
				reportLine.OrderPaymentTime.Format(model.ReportLineTimeFormat),
				reportLine.OrderSettlementTime.Format(model.ReportLineTimeFormat),
				strconv.FormatInt(reportLine.OrderDetailID, 10),
				strconv.FormatInt(reportLine.ProductID, 10),
				strconv.FormatInt(reportLine.CategoryID, 10),
				strconv.FormatFloat(reportLine.ProductPriceAmount, 'f', 2, 64),
				strconv.FormatFloat(reportLine.PromoAmount, 'f', 2, 64),
				strconv.FormatFloat(reportLine.FeeBaseAmount, 'f', 2, 64),
				strconv.FormatFloat(reportLine.FeeFinalAmount, 'f', 2, 64),
			}); writerError != nil {
				if closeErr := reportFileWriter.CloseWithError(writerError); closeErr != nil {
					return closeErr
				}

				return writerError
			}
		}
	})

	reportExporterErrGroup.Go(func() error {
		if err := reportCSVEg.Wait(); err != nil {
			reportFileWriter.CloseWithError(err)
			return err
		}

		reportFileCSVWriter.Flush()
		reportFileWriter.Close()
		return nil
	})

	return reportFileReader, nil
}
