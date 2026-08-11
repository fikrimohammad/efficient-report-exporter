package api

import (
	"context"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	apimodel "github.com/fikrimohammad/efficient-report-exporter/model/api"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

const (
	maxInputLength = 64
	maxTimeRange   = 90 * 24 * time.Hour
)

func (h *Handler) RequestExportReport(ctx context.Context, c *app.RequestContext) {
	var (
		req    apimodel.ExportReportRequest
		err    error
		result *usecase.RequestExportReportResult
	)

	defer func() {
		if err != nil {
			_ = c.Error(err)
			code := errs.CodeFromError(err)
			msg := err.Error()
			if code.HTTPStatus() >= 500 {
				msg = "internal server error"
			}
			c.JSON(
				code.HTTPStatus(),
				apimodel.ExportReportResponse{
					Base: &apimodel.BaseResponse{
						Code:    codeToString(code),
						Message: msg,
					},
				},
			)
			return
		}

		c.JSON(200, apimodel.ExportReportResponse{
			Base: &apimodel.BaseResponse{Code: codeToString(errs.OK), Message: "success"},
			Data: &apimodel.ExportReportData{JobID: strconv.FormatInt(result.JobID, 10)},
		})
	}()

	if err = c.BindAndValidate(&req); err != nil {
		err = errs.Wrap(errs.InvalidArgument, "invalid request body", err)
		return
	}

	if len(req.RequestID) > maxInputLength {
		err = errs.New(errs.InvalidArgument, "request_id exceeds max length")
		return
	}
	requestID, parseErr := strconv.ParseInt(req.RequestID, 10, 64)
	if parseErr != nil {
		err = errs.Wrap(errs.InvalidArgument, "invalid request_id format", parseErr)
		return
	}
	if requestID <= 0 {
		err = errs.New(errs.InvalidArgument, "request_id must be positive")
		return
	}

	if len(req.ShopID) > maxInputLength {
		err = errs.New(errs.InvalidArgument, "shop_id exceeds max length")
		return
	}
	shopID, parseErr := strconv.ParseInt(req.ShopID, 10, 64)
	if parseErr != nil {
		err = errs.Wrap(errs.InvalidArgument, "invalid shop_id format", parseErr)
		return
	}
	if shopID <= 0 {
		err = errs.New(errs.InvalidArgument, "shop_id must be positive")
		return
	}

	if len(req.StartTime) > maxInputLength {
		err = errs.New(errs.InvalidArgument, "start_time exceeds max length")
		return
	}
	startTime, parseErr := time.Parse(time.RFC3339, req.StartTime)
	if parseErr != nil {
		err = errs.Wrap(errs.InvalidArgument, "invalid start_time format", parseErr)
		return
	}

	if len(req.EndTime) > maxInputLength {
		err = errs.New(errs.InvalidArgument, "end_time exceeds max length")
		return
	}
	endTime, parseErr := time.Parse(time.RFC3339, req.EndTime)
	if parseErr != nil {
		err = errs.Wrap(errs.InvalidArgument, "invalid end_time format", parseErr)
		return
	}

	if !endTime.After(startTime) {
		err = errs.New(errs.InvalidArgument, "end_time must be after start_time")
		return
	}
	if endTime.Sub(startTime) > maxTimeRange {
		err = errs.New(errs.InvalidArgument, "time range must not exceed 90 days")
		return
	}

	result, err = h.reportUseCase.RequestExportReport(ctx, usecase.RequestExportReportParams{
		RequestID: requestID,
		ShopID:    shopID,
		StartTime: startTime,
		EndTime:   endTime,
	})
}
