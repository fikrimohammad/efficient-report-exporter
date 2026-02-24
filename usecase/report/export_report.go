package report

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-typedpipe"
	"golang.org/x/sync/errgroup"
)

func (u *useCase) ExportReport(ctx context.Context, params usecase.ExportReportParams) (*usecase.ExportReportResult, error) {
	rg := reportExporter{
		reportMySQLRepository: u.reportMySQLRepository,
		ctx:                   ctx,
		params:                params,
	}

	return rg.Export()
}

type reportExporter struct {
	reportMySQLRepository repository.ReportMySQL
	ctx                   context.Context
	params                usecase.ExportReportParams
}

func (rg *reportExporter) Export() (*usecase.ExportReportResult, error) {
	if err := rg.validateParams(); err != nil {
		return nil, err
	}

	reportDataStream, err := rg.asyncFetchReports()
	if err != nil {
		return nil, err
	}

	reportLineDataStream, err := rg.asyncBuildReportLine(reportDataStream)
	if err != nil {
		return nil, err
	}

	reportCSVFileDataStream, err := rg.asyncBuildReportCSVFile(reportLineDataStream)
	if err != nil {
		return nil, err
	}

	result := &usecase.ExportReportResult{
		FileName: fmt.Sprintf(
			"%s_%s_%s.csv",
			rg.params.StartTime.Format(model.ReportNameTimeFormat),
			rg.params.EndTime.Format(model.ReportNameTimeFormat),
		),
		File: reportCSVFileDataStream,
	}

	return result, nil
}

func (rg *reportExporter) validateParams() error {
	if rg.params.ShopID == 0 {
		return errors.New("shop_id is required")
	}

	if rg.params.StartTime.IsZero() {
		return errors.New("start_time is required")
	}

	if rg.params.EndTime.IsZero() {
		return errors.New("end_time is required")
	}

	if rg.params.StartTime.After(rg.params.EndTime) {
		return errors.New("start time is after end time")
	}

	return nil
}

func (rg *reportExporter) asyncFetchReports() (typedpipe.Reader[model.Report], error) {
	return rg.reportMySQLRepository.AsyncQueryReport(rg.ctx, repository.QueryReportFilter{
		ShopID: &rg.params.ShopID,
		OrderSettlementTimeRange: &repository.QueryReportTimeRange{
			StartTime: &rg.params.StartTime,
			EndTime:   &rg.params.EndTime,
		},
	})
}

func (rg *reportExporter) asyncBuildReportLine(reportDataStream typedpipe.Reader[model.Report]) (typedpipe.Reader[model.ReportLine], error) {
	var (
		reportLineWriter, reportLineReader, _     = typedpipe.New[model.ReportLine]()
		reportLineWorkerEg, reportLineWorkerEgCtx = errgroup.WithContext(rg.ctx)
		reportLineWorkerPoolCount                 = 32
	)

	reportLineWorkerEg.SetLimit(reportLineWorkerPoolCount)
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

	go func() {
		defer func() {
			if r := recover(); r != nil {
				reportLineWriter.CloseWithError(fmt.Errorf("%v", r))
			}
		}()

		if err := reportLineWorkerEg.Wait(); err != nil {
			reportLineWriter.CloseWithError(err)
			return
		}

		reportLineWriter.Close()
	}()

	return reportLineReader, nil
}

func (rg *reportExporter) asyncBuildReportCSVFile(reportLineDataStream typedpipe.Reader[model.ReportLine]) (io.ReadCloser, error) {
	var (
		reportFileReader, reportFileWriter = io.Pipe()
		reportFileCSVWriter                = csv.NewWriter(bufio.NewWriterSize(reportFileWriter, 1024*1024))
		reportCSVEg, reportCSVEgCtx        = errgroup.WithContext(rg.ctx)
	)

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
		return nil, err
	}

	reportCSVEg.SetLimit(1)
	reportCSVEg.Go(func() error {
		defer func() {
			if r := recover(); r != nil {
				reportFileWriter.CloseWithError(fmt.Errorf("%v", r))
			}
		}()

		for {
			reportLine, err := reportLineDataStream.Read(reportCSVEgCtx)
			if err != nil {
				if !errors.Is(err, typedpipe.ErrPipeClosed) {
					if closeErr := reportFileWriter.CloseWithError(err); closeErr != nil {
						return closeErr
					}

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
				if closeErr := reportFileWriter.CloseWithError(err); closeErr != nil {
					return closeErr
				}

				return writerError
			}
		}
	})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				reportFileWriter.CloseWithError(fmt.Errorf("%v", r))
			}
		}()

		if err := reportCSVEg.Wait(); err != nil {
			reportFileWriter.CloseWithError(err)
			return
		}

		reportFileWriter.Close()
	}()

	return reportFileReader, nil
}
