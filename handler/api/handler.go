package api

import (
	"strconv"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

type Handler struct {
	reportUseCase usecase.Report
}

func New(reportUseCase usecase.Report) (*Handler, error) {
	if reportUseCase == nil {
		return nil, errs.New(apperrors.Internal, "report use case is not initialized")
	}

	return &Handler{
		reportUseCase: reportUseCase,
	}, nil
}

// codeToString converts an apperrors.Code to its string representation for API responses.
func codeToString(code apperrors.Code) string {
	return strconv.Itoa(int(code))
}

// formatAPITime renders a timestamp in the API format, returning an empty
// string for the zero time (e.g. a job that has not been updated yet).
func formatAPITime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(constant.APITimeFormat)
}
