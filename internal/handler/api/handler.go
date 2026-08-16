package api

import (
	"strconv"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
)

type Handler struct {
	reportUseCase usecase.Report
}

func New(reportUseCase usecase.Report) (*Handler, error) {
	if reportUseCase == nil {
		return nil, errs.New(constant.Internal, "report use case is not initialized")
	}

	return &Handler{
		reportUseCase: reportUseCase,
	}, nil
}

// codeToString converts an error code to its string representation for API responses.
func codeToString(code int) string {
	return strconv.Itoa(code)
}

// formatAPITime renders a timestamp in the API format, returning an empty
// string for the zero time (e.g. a job that has not been updated yet).
func formatAPITime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(constant.APITimeFormat)
}
