package api

import (
	"strconv"

	"github.com/fikrimohammad/efficient-report-exporter/config"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-dev-sdk/confloader"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

type Handler struct {
	reportUseCase usecase.Report
	dynamicConfig *confloader.Loader[config.DynamicConfig]
}

func New(reportUseCase usecase.Report, dynamicConfig *confloader.Loader[config.DynamicConfig]) (*Handler, error) {
	if reportUseCase == nil {
		return nil, errs.New(errs.Internal, "report use case is not initialized")
	}

	return &Handler{
		reportUseCase: reportUseCase,
		dynamicConfig: dynamicConfig,
	}, nil
}

// DynamicConfig returns the dynamic config loader.
func (h *Handler) DynamicConfig() *confloader.Loader[config.DynamicConfig] {
	return h.dynamicConfig
}

// codeToString converts an errs.Code to its string representation for API responses.
func codeToString(code errs.Code) string {
	return strconv.Itoa(int(code))
}
