package mq

import (
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
