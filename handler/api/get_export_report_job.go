package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	apimodel "github.com/fikrimohammad/efficient-report-exporter/model/api"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

func (h *Handler) GetExportReportJob(ctx context.Context, c *app.RequestContext) {
	var (
		jobIDStr = c.Param("job_id")
		err      error
		result   *usecase.GetExportReportJobResult
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
				apimodel.GetExportReportJobResponse{
					Base: &apimodel.BaseResponse{
						Code:    codeToString(code),
						Message: msg,
					},
				},
			)
			return
		}

		c.JSON(http.StatusOK, apimodel.GetExportReportJobResponse{
			Base: &apimodel.BaseResponse{Code: codeToString(errs.OK), Message: "success"},
			Data: &apimodel.GetExportReportJobData{
				JobID:        strconv.FormatInt(result.JobID, 10),
				Status:       result.Status,
				DownloadURL:  result.DownloadURL,
				ErrorMessage: result.ErrorMessage,
				CreatedAt:    result.CreationTime.Format(constant.APITimeFormat),
				UpdatedAt:    result.UpdateTime.Format(constant.APITimeFormat),
			},
		})
	}()

	if len(jobIDStr) > maxInputLength {
		err = errs.New(errs.InvalidArgument, "job_id exceeds max length")
		return
	}
	jobID, parseErr := strconv.ParseInt(jobIDStr, 10, 64)
	if parseErr != nil {
		err = errs.Wrap(errs.InvalidArgument, "invalid job_id format", parseErr)
		return
	}

	if jobID <= 0 {
		err = errs.New(errs.InvalidArgument, "job_id must be positive")
		return
	}

	result, err = h.reportUseCase.GetExportReportJob(ctx, usecase.GetExportReportJobParams{
		JobID: jobID,
	})
}
