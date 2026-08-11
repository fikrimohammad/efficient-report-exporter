package mysql

import (
	"context"
	"strings"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

func TestBuildInsertExportReportJobQuery(t *testing.T) {
	r := newTestRepo(t)

	params := repository.InsertExportReportJobParams{
		RequestID: 1,
		ShopID:    100,
		StartTime: 1000,
		EndTime:   2000,
		Status:    constant.ExportReportJobStatusProcessing,
	}

	query, args := r.buildInsertExportReportJobQuery(context.Background(), 42, params)

	if !strings.Contains(query, "INSERT INTO export_report_job") {
		t.Fatal("should be an INSERT query")
	}

	if len(args) == 0 {
		t.Fatal("expected non-empty args")
	}
}
