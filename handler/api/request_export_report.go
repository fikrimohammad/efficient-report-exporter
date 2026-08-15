package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	apimodel "github.com/fikrimohammad/efficient-report-exporter/model/api"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

const maxInputLength = 64

func (h *Handler) RequestExportReport(ctx context.Context, c *app.RequestContext) {
	var (
		req    apimodel.ExportReportRequest
		err    error
		result *usecase.RequestExportReportResult
	)

	defer func() {
		if err != nil {
			_ = c.Error(err)
			code := apperrors.CodeFromError(err)
			msg := err.Error()
			if apperrors.HTTPStatus(code) >= 500 {
				msg = "internal server error"
			}
			c.JSON(
				apperrors.HTTPStatus(code),
				apimodel.ExportReportResponse{
					Base: &apimodel.BaseResponse{
						Code:    codeToString(code),
						Message: msg,
					},
				},
			)
			return
		}

		c.JSON(http.StatusOK, apimodel.ExportReportResponse{
			Base: &apimodel.BaseResponse{Code: codeToString(apperrors.OK), Message: "success"},
			Data: &apimodel.ExportReportData{JobID: strconv.FormatInt(result.JobID, 10)},
		})
	}()

	if err = c.BindAndValidate(&req); err != nil {
		err = errs.Wrap(apperrors.InvalidArgument, "invalid request body", err)
		return
	}

	if len(req.RequestID) > maxInputLength {
		err = errs.New(apperrors.InvalidArgument, "request_id exceeds max length")
		return
	}
	requestID, parseErr := strconv.ParseInt(req.RequestID, 10, 64)
	if parseErr != nil {
		err = errs.Wrap(apperrors.InvalidArgument, "invalid request_id format", parseErr)
		return
	}
	if requestID <= 0 {
		err = errs.New(apperrors.InvalidArgument, "request_id must be positive")
		return
	}

	if len(req.ShopID) > maxInputLength {
		err = errs.New(apperrors.InvalidArgument, "shop_id exceeds max length")
		return
	}
	shopID, parseErr := strconv.ParseInt(req.ShopID, 10, 64)
	if parseErr != nil {
		err = errs.Wrap(apperrors.InvalidArgument, "invalid shop_id format", parseErr)
		return
	}
	if shopID <= 0 {
		err = errs.New(apperrors.InvalidArgument, "shop_id must be positive")
		return
	}

	if len(req.StartTime) > maxInputLength {
		err = errs.New(apperrors.InvalidArgument, "start_time exceeds max length")
		return
	}
	startTime, parseErr := time.Parse(time.RFC3339, req.StartTime)
	if parseErr != nil {
		err = errs.Wrap(apperrors.InvalidArgument, "invalid start_time format", parseErr)
		return
	}

	if len(req.EndTime) > maxInputLength {
		err = errs.New(apperrors.InvalidArgument, "end_time exceeds max length")
		return
	}
	endTime, parseErr := time.Parse(time.RFC3339, req.EndTime)
	if parseErr != nil {
		err = errs.Wrap(apperrors.InvalidArgument, "invalid end_time format", parseErr)
		return
	}

	if !endTime.After(startTime) {
		err = errs.New(apperrors.InvalidArgument, "end_time must be after start_time")
		return
	}
	if endTime.Sub(startTime) > constant.MaxExportTimeRange {
		err = errs.New(apperrors.InvalidArgument, "time range must not exceed 90 days")
		return
	}

	result, err = h.reportUseCase.RequestExportReport(ctx, usecase.RequestExportReportParams{
		RequestID: requestID,
		ShopID:    shopID,
		StartTime: startTime,
		EndTime:   endTime,
	})
}
