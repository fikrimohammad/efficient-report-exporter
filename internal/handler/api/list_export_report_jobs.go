package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	apimodel "github.com/fikrimohammad/efficient-report-exporter/internal/model/api"
	"github.com/fikrimohammad/efficient-report-exporter/internal/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
)

func (h *Handler) ListExportReportJobs(ctx context.Context, c *app.RequestContext) {
	var (
		err           error
		result        *usecase.ListExportReportJobsResult
		jobs          []*apimodel.ExportReportJobSummary
		nextPageToken string
	)

	defer func() {
		if err != nil {
			_ = c.Error(err)
			code := constant.CodeFromError(err)
			msg := err.Error()
			if constant.HTTPStatus(code) >= 500 {
				msg = "internal server error"
			}
			c.JSON(
				constant.HTTPStatus(code),
				apimodel.ListExportReportJobsResponse{
					Base: &apimodel.BaseResponse{
						Code:    codeToString(code),
						Message: msg,
					},
				},
			)
			return
		}

		c.JSON(http.StatusOK, apimodel.ListExportReportJobsResponse{
			Base: &apimodel.BaseResponse{Code: codeToString(constant.OK), Message: "success"},
			Data: &apimodel.ListExportReportJobsData{
				Jobs:          jobs,
				NextPageToken: nextPageToken,
			},
		})
	}()

	shopIDStr := c.Query("shop_id")
	if shopIDStr == "" {
		err = errs.New(constant.InvalidArgument, "shop_id is required")
		return
	}
	if len(shopIDStr) > maxInputLength {
		err = errs.New(constant.InvalidArgument, "shop_id exceeds max length")
		return
	}
	shopID, parseErr := strconv.ParseInt(shopIDStr, 10, 64)
	if parseErr != nil {
		err = errs.Wrap(constant.InvalidArgument, "invalid shop_id format", parseErr)
		return
	}
	if shopID <= 0 {
		err = errs.New(constant.InvalidArgument, "shop_id must be positive")
		return
	}

	var pageToken int64
	if pageTokenStr := c.Query("page_token"); pageTokenStr != "" {
		if len(pageTokenStr) > maxInputLength {
			err = errs.New(constant.InvalidArgument, "page_token exceeds max length")
			return
		}
		pageToken, parseErr = strconv.ParseInt(pageTokenStr, 10, 64)
		if parseErr != nil {
			err = errs.Wrap(constant.InvalidArgument, "invalid page_token format", parseErr)
			return
		}
	}

	var limit int
	if limitStr := c.Query("limit"); limitStr != "" {
		if len(limitStr) > maxInputLength {
			err = errs.New(constant.InvalidArgument, "limit exceeds max length")
			return
		}
		limitVal, parseErr := strconv.Atoi(limitStr)
		if parseErr != nil {
			err = errs.Wrap(constant.InvalidArgument, "invalid limit format", parseErr)
			return
		}
		if limitVal <= 0 || limitVal > constant.MaxListExportReportJobsLimit {
			err = errs.New(constant.InvalidArgument, "limit must be between 1 and 100")
			return
		}
		limit = limitVal
	}

	result, err = h.reportUseCase.ListExportReportJobs(ctx, usecase.ListExportReportJobsParams{
		ShopID:    shopID,
		PageToken: pageToken,
		Limit:     limit,
	})
	if err != nil {
		return
	}

	jobs = make([]*apimodel.ExportReportJobSummary, 0, len(result.Jobs))
	for _, job := range result.Jobs {
		jobs = append(jobs, &apimodel.ExportReportJobSummary{
			JobID:     strconv.FormatInt(job.JobID, 10),
			Status:    string(job.Status),
			StartTime: job.StartTime.Format(constant.APITimeFormat),
			EndTime:   job.EndTime.Format(constant.APITimeFormat),
			CreatedAt: job.CreationTime.Format(constant.APITimeFormat),
			UpdatedAt: formatAPITime(job.UpdateTime),
		})
	}

	if result.NextPageToken > 0 {
		nextPageToken = strconv.FormatInt(result.NextPageToken, 10)
	}
}
