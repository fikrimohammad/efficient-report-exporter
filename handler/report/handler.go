package report

import "github.com/fikrimohammad/efficient-report-exporter/usecase"

type Handler struct {
	reportUseCase usecase.Report
}

func New(reportUseCase usecase.Report) *Handler {
	return &Handler{
		reportUseCase: reportUseCase,
	}
}
