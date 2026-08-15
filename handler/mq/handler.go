package mq

import (
	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
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
