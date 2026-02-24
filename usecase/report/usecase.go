package report

import (
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
)

type useCase struct {
	reportMySQLRepository repository.ReportMySQL
}

func New(reportMySQLRepository repository.ReportMySQL) usecase.Report {
	return &useCase{
		reportMySQLRepository: reportMySQLRepository,
	}
}
