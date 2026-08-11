package mq

import (
	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
)

type Handler struct {
	reportUseCase usecase.Report
}

func New(reportUseCase usecase.Report) (*Handler, error) {
	if reportUseCase == nil {
		return nil, errs.New(errs.Internal, "report use case is not initialized")
	}

	return &Handler{
		reportUseCase: reportUseCase,
	}, nil
}
