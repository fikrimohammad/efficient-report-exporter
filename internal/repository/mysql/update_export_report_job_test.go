package mysql

import (
	"strings"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
)

func TestBuildUpdateExportReportJobQuery(t *testing.T) {
	r := newTestRepo(t)

	params := repository.UpdateExportReportJobParams{
		JobID:  42,
		Status: constant.ExportReportJobStatusFailed,
	}

	query, args := r.buildUpdateExportReportJobQuery(params)

	if !strings.Contains(query, "UPDATE export_report_job") {
		t.Fatal("should be an UPDATE query")
	}

	if !strings.Contains(query, "status = :status") || !strings.Contains(query, "extra = :extra") {
		t.Fatal("UPDATE should set status and extra")
	}

	if !strings.Contains(query, "id = :id") {
		t.Fatal("UPDATE should filter by id")
	}

	if len(args) == 0 {
		t.Fatal("expected non-empty args")
	}
}
