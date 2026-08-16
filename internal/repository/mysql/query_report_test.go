package mysql

import (
	"strings"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
)

func TestBuildReportQuery(t *testing.T) {
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	shopID := int64(100)

	tests := []struct {
		name   string
		filter repository.QueryReportFilter
		check  func(t *testing.T, query string)
	}{
		{
			name: "no filters returns base query",
			check: func(t *testing.T, query string) {
				if !strings.Contains(query, "FROM") || !strings.Contains(query, "report") {
					t.Fatal("query should select from report table")
				}
				if !strings.Contains(query, "SELECT") || !strings.Contains(query, "id") {
					t.Fatal("query should select fields")
				}
				if strings.Contains(query, "WHERE") {
					t.Fatal("no filters should not have WHERE clause")
				}
				if strings.Contains(query, "LIMIT") {
					t.Fatal("no limit should not have LIMIT clause")
				}
				if strings.Contains(query, "ORDER BY") {
					t.Fatal("no filters should not have ORDER BY")
				}
			},
		},
		{
			name: "shop_id filter",
			filter: repository.QueryReportFilter{
				ShopID: &shopID,
				Limit:  10,
			},
			check: func(t *testing.T, query string) {
				if !strings.Contains(query, "shop_id = :shop_id") {
					t.Fatal("query should filter by shop_id")
				}
				if !strings.Contains(query, "LIMIT") {
					t.Fatal("query should have LIMIT")
				}
				if strings.Count(query, "LIMIT") != 1 {
					t.Fatal("should have exactly one LIMIT clause")
				}
			},
		},
		{
			name: "time range filter",
			filter: repository.QueryReportFilter{
				OrderSettlementTimeRange: &repository.QueryReportTimeRange{
					StartTime: &now,
					EndTime:   &now,
				},
			},
			check: func(t *testing.T, query string) {
				if !strings.Contains(query, "order_settlement_time") {
					t.Fatal("query should filter by order_settlement_time")
				}
			},
		},
		{
			name: "cursor pagination",
			filter: repository.QueryReportFilter{
				LastOrderSettlementTime: &now,
				LastReportID:            500,
				Limit:                   100,
			},
			check: func(t *testing.T, query string) {
				if !strings.Contains(query, "order_settlement_time > :last_order_settlement_time") {
					t.Fatal("cursor pagination should use order_settlement_time > :last_order_settlement_time")
				}
				if !strings.Contains(query, "id > :last_report_id") {
					t.Fatal("cursor pagination should use id > :last_report_id")
				}
			},
		},
		{
			name: "all filters combined",
			filter: repository.QueryReportFilter{
				ShopID: &shopID,
				OrderSettlementTimeRange: &repository.QueryReportTimeRange{
					StartTime: &now,
				},
				LastOrderSettlementTime: &now,
				LastReportID:            999,
				Limit:                   50,
			},
			check: func(t *testing.T, query string) {
				if !strings.Contains(query, "shop_id = :shop_id") {
					t.Fatal("should filter by shop_id")
				}
				if !strings.Contains(query, "order_settlement_time") {
					t.Fatal("should filter by order_settlement_time")
				}
				if !strings.Contains(query, "id > :last_report_id") {
					t.Fatal("should filter by id > :last_report_id")
				}
				if !strings.Contains(query, "ORDER BY order_settlement_time ASC, id ASC") {
					t.Fatal("should order by order_settlement_time, id for keyset pagination")
				}
			},
		},
	}

	r := newTestRepo(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, _ := r.buildReportQuery(tt.filter)
			tt.check(t, query)
		})
	}
}
