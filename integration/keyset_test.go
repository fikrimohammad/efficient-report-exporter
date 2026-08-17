//go:build integration

package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/model"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
)

// TestKeysetPaginationReturnsCorrectRows verifies the composite
// (order_settlement_time, id) keyset cursor against a real MySQL: every row in
// the range is returned exactly once, in (settlement, id) order, with correct
// handling of settlement-time ties and the exclusive end boundary — across page
// boundaries that fall *inside* a tie group.
//
// It drives the repository directly, mirroring the cursor-advancement loop of
// the report pipeline's fetch stage (last row's (settlement_time, id) becomes
// the next page's cursor).
func TestKeysetPaginationReturnsCorrectRows(t *testing.T) {
	ctx := context.Background()
	d := setupDeps(t)

	shopID := int64(999999)
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(3 * time.Hour)

	seedKeysetRows(t, d.rawDB, shopID, start)

	expected := queryOrderedRows(t, d.rawDB, shopID, start, end)
	if len(expected) == 0 {
		t.Fatal("seeded rows not found")
	}

	const pageSize = 4 // first page boundary lands inside the 5-row tie at start
	var (
		lastID     int64
		lastSettle time.Time
		hasCursor  bool
		got        []*model.Report
	)

	for {
		rows, err := d.mysqlRepo.QueryReport(ctx, repository.QueryReportFilter{
			ShopID: &shopID,
			OrderSettlementTimeRange: &repository.QueryReportTimeRange{
				StartTime: &start,
				EndTime:   &end,
			},
			Limit:                   pageSize,
			LastReportID:            lastID,
			LastOrderSettlementTime: lastSettle,
			HasCursor:               hasCursor,
		})
		if err != nil {
			t.Fatalf("query page: %v", err)
		}
		if len(rows) == 0 {
			break
		}
		got = append(got, rows...)

		last := rows[len(rows)-1]
		lastID = last.ID
		lastSettle = time.UnixMilli(last.OrderSettlementTime)
		hasCursor = true

		if len(rows) < pageSize {
			break
		}
	}

	if len(got) != len(expected) {
		t.Fatalf("row count mismatch: got %d, want %d", len(got), len(expected))
	}
	seen := make(map[int64]bool, len(got))
	for i := range got {
		if seen[got[i].ID] {
			t.Fatalf("duplicate row id %d", got[i].ID)
		}
		seen[got[i].ID] = true
		if got[i].ID != expected[i].ID || got[i].OrderSettlementTime != expected[i].Settle.UnixMilli() {
			t.Fatalf("row %d mismatch: got id=%d settle=%v, want id=%d settle=%v",
				i, got[i].ID, got[i].OrderSettlementTime, expected[i].ID, expected[i].Settle)
		}
	}
}

type orderedRow struct {
	ID     int64
	Settle time.Time
}

// seedKeysetRows inserts a deterministic set of rows for the shop. Settlement
// times are deliberately inserted *out of order* (so id order ≠ settlement
// order), with a 5-row settlement tie at the start (larger than the page size)
// and one row exactly at `end` to verify the exclusive end boundary excludes it.
func seedKeysetRows(t *testing.T, rawDB *sql.DB, shopID int64, start time.Time) {
	t.Helper()
	ctx := context.Background()

	if _, err := rawDB.ExecContext(ctx, "DELETE FROM report WHERE shop_id = ?", shopID); err != nil {
		t.Fatalf("delete existing rows: %v", err)
	}

	settlements := []time.Time{
		start.Add(1 * time.Hour),    // id 1
		start,                       // id 2
		start.Add(90 * time.Minute), // id 3
		start,                       // id 4
		start.Add(3 * time.Hour),    // id 5  (exactly == end)
		start,                       // id 6
		start.Add(1 * time.Hour),    // id 7
		start,                       // id 8
		start,                       // id 9
	}

	for i, settle := range settlements {
		_, err := rawDB.ExecContext(ctx,
			`INSERT INTO report (shop_id, order_id, order_creation_time, order_payment_time, order_settlement_time, fee_id, details, creation_time, update_time)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			shopID, int64(i+1), settle.UnixMilli(), settle.UnixMilli(), settle.UnixMilli(), int64(i+1), `[]`, settle.UnixMilli(), settle.UnixMilli())
		if err != nil {
			t.Fatalf("insert row %d: %v", i, err)
		}
	}
}

func queryOrderedRows(t *testing.T, rawDB *sql.DB, shopID int64, start, end time.Time) []orderedRow {
	t.Helper()
	rows, err := rawDB.QueryContext(context.Background(),
		`SELECT id, order_settlement_time FROM report
		 WHERE shop_id = ? AND order_settlement_time >= ? AND order_settlement_time < ?
		 ORDER BY order_settlement_time ASC, id ASC`,
		shopID, start.UnixMilli(), end.UnixMilli())
	if err != nil {
		t.Fatalf("query expected rows: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var out []orderedRow
	for rows.Next() {
		var (
			r        orderedRow
			settleMs int64
		)
		if err := rows.Scan(&r.ID, &settleMs); err != nil {
			t.Fatalf("scan expected row: %v", err)
		}
		r.Settle = time.UnixMilli(settleMs)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate expected rows: %v", err)
	}
	return out
}
