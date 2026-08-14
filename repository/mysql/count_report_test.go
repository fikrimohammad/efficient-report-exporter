package mysql

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

func TestBuildCountReportQuery(t *testing.T) {
	r := &repo{}

	shopID := int64(100)
	start := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC)

	query, args := r.buildCountReportQuery(context.Background(), repository.QueryReportFilter{
		ShopID: &shopID,
		OrderSettlementTimeRange: &repository.QueryReportTimeRange{
			StartTime: &start,
			EndTime:   &end,
		},
	})

	if !strings.Contains(query, "COUNT(*)") {
		t.Fatal("count query should use COUNT(*)")
	}
	if !strings.Contains(query, "shop_id = :shop_id") {
		t.Fatal("count query should filter by shop_id")
	}
	if !strings.Contains(query, "order_settlement_time") {
		t.Fatal("count query should filter by order_settlement_time")
	}
	if v, ok := args["shop_id"].(int64); !ok || v != shopID {
		t.Fatalf("expected shop_id arg %d, got %v", shopID, args["shop_id"])
	}

	noFilterQuery, noFilterArgs := r.buildCountReportQuery(context.Background(), repository.QueryReportFilter{})
	if strings.Contains(noFilterQuery, "WHERE") {
		t.Fatal("no filters should not have WHERE clause")
	}
	if len(noFilterArgs) != 0 {
		t.Fatalf("no filters should have no args, got %v", noFilterArgs)
	}
}
