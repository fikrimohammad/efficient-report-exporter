package mysql

import (
	"strings"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

func TestBuildExportReportJobQuery(t *testing.T) {
	tests := []struct {
		name   string
		filter repository.QueryExportReportJobFilter
		check  func(t *testing.T, query string)
	}{
		{
			name: "no filters returns base query",
			check: func(t *testing.T, query string) {
				if !strings.Contains(query, "FROM") || !strings.Contains(query, "export_report_job") {
					t.Fatal("should select from export_report_job table")
				}
				if strings.Contains(query, "WHERE") {
					t.Fatal("no filters should not add WHERE")
				}
			},
		},
		{
			name: "filter by job_id",
			filter: repository.QueryExportReportJobFilter{
				JobID: 42,
				Limit: 1,
			},
			check: func(t *testing.T, query string) {
				if !strings.Contains(query, "id = :job_id") {
					t.Fatal("should filter by id (job_id)")
				}
				if !strings.Contains(query, "LIMIT") {
					t.Fatal("should have LIMIT")
				}
			},
		},
		{
			name: "filter by request_id",
			filter: repository.QueryExportReportJobFilter{
				RequestID: 100,
			},
			check: func(t *testing.T, query string) {
				if !strings.Contains(query, "request_id = :request_id") {
					t.Fatal("should filter by request_id")
				}
			},
		},
		{
			name: "filter by shop_id",
			filter: repository.QueryExportReportJobFilter{
				ShopID: 200,
			},
			check: func(t *testing.T, query string) {
				if !strings.Contains(query, "shop_id = :shop_id") {
					t.Fatal("should filter by shop_id")
				}
			},
		},
		{
			name: "cursor pagination",
			filter: repository.QueryExportReportJobFilter{
				LastExportReportJobID: 50,
				Limit:                 10,
			},
			check: func(t *testing.T, query string) {
				if !strings.Contains(query, "id > :last_export_report_job_id") {
					t.Fatal("should filter by id > :last_export_report_job_id for cursor pagination")
				}
			},
		},
	}

	r := newTestRepo(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, _ := r.buildExportReportJobQuery(tt.filter)
			tt.check(t, query)
		})
	}
}
