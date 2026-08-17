package report

import (
	"bufio"
	"bytes"
	stdcsv "encoding/csv"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	zerocsv "github.com/fikrimohammad/go-zerocsv"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/model"
)

func testReportLine(shopID, feeID, orderID int64) model.ReportLine {
	return model.ReportLine{
		ShopID:              shopID,
		OrderID:             orderID,
		OrderCreationTime:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).UnixMilli(),
		OrderPaymentTime:    time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC).UnixMilli(),
		OrderSettlementTime: time.Date(2026, 1, 2, 3, 4, 7, 0, time.UTC).UnixMilli(),
		FeeID:               feeID,
		ReportFeeDetail: model.ReportFeeDetail{
			OrderDetailID:      1,
			ProductID:          2,
			CategoryID:         3,
			ProductPriceAmount: 4.5,
			PromoAmount:        1.25,
			FeeBaseAmount:      6.5,
			FeeFinalAmount:     7.75,
		},
	}
}

// writeHeaderWith writes fields as one CSV record, exercising the same quoting
// path as writeHeader on arbitrary fields.
func (b *reportCSVFileBuilder) writeHeaderWith(fields []string) error {
	row := b.row[:0]
	for i := range fields {
		row = append(row, zerocsv.ColumnString(fields[i]))
	}
	return b.w.Write(row...)
}

// referenceCSVRow renders rl exactly as encoding/csv's default Writer would for
// the report's 13 columns. It is the ground truth that the zerocsv-backed
// builder must match byte-for-byte.
func referenceCSVRow(rl model.ReportLine) string {
	var b bytes.Buffer
	cw := stdcsv.NewWriter(&b)
	_ = cw.Write([]string{
		strconv.FormatInt(rl.ShopID, 10),
		strconv.FormatInt(rl.FeeID, 10),
		strconv.FormatInt(rl.OrderID, 10),
		time.UnixMilli(rl.OrderCreationTime).UTC().Format(constant.ReportLineTimeFormat),
		time.UnixMilli(rl.OrderPaymentTime).UTC().Format(constant.ReportLineTimeFormat),
		time.UnixMilli(rl.OrderSettlementTime).UTC().Format(constant.ReportLineTimeFormat),
		strconv.FormatInt(rl.OrderDetailID, 10),
		strconv.FormatInt(rl.ProductID, 10),
		strconv.FormatInt(rl.CategoryID, 10),
		strconv.FormatFloat(rl.ProductPriceAmount, 'f', -1, 64),
		strconv.FormatFloat(rl.PromoAmount, 'f', -1, 64),
		strconv.FormatFloat(rl.FeeBaseAmount, 'f', -1, 64),
		strconv.FormatFloat(rl.FeeFinalAmount, 'f', -1, 64),
	})
	cw.Flush()
	return b.String()
}

// TestReportCSVFileBuilder_MatchesEncodingCSV proves the builder's output is
// byte-identical to encoding/csv's default Writer for the real header row and
// data rows, including int64/float64 formatting edge cases.
func TestReportCSVFileBuilder_MatchesEncodingCSV(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	rows := []model.ReportLine{
		testReportLine(100, 200, 300),
		testReportLine(100, 201, 301),
		testReportLine(101, 202, 302),
		{
			ShopID:              math.MaxInt64,
			FeeID:               42,
			OrderID:             1,
			OrderCreationTime:   ts.UnixMilli(),
			OrderPaymentTime:    ts.UnixMilli(),
			OrderSettlementTime: ts.UnixMilli(),
			ReportFeeDetail: model.ReportFeeDetail{
				OrderDetailID:      1,
				ProductID:          2,
				CategoryID:         3,
				ProductPriceAmount: 9.99,
				PromoAmount:        1,
				FeeBaseAmount:      8.99,
				FeeFinalAmount:     0.5,
			},
		},
		{
			ShopID:              -1,
			FeeID:               -2,
			OrderID:             -3,
			OrderCreationTime:   ts.Add(-24 * time.Hour).UnixMilli(),
			OrderPaymentTime:    ts.Add(-23 * time.Hour).UnixMilli(),
			OrderSettlementTime: ts.Add(-22 * time.Hour).UnixMilli(),
			ReportFeeDetail: model.ReportFeeDetail{
				OrderDetailID:      -4,
				ProductID:          -5,
				CategoryID:         -6,
				ProductPriceAmount: -0.5,
				PromoAmount:        -1.25,
				FeeBaseAmount:      -9.99,
				FeeFinalAmount:     -0.1,
			},
		},
		{
			ShopID:              100,
			FeeID:               10,
			OrderID:             1000,
			OrderCreationTime:   time.Date(2026, 8, 16, 22, 47, 59, 0, time.UTC).UnixMilli(),
			OrderPaymentTime:    time.Date(2026, 8, 16, 22, 48, 59, 0, time.UTC).UnixMilli(),
			OrderSettlementTime: time.Date(2026, 8, 16, 22, 49, 59, 0, time.UTC).UnixMilli(),
			ReportFeeDetail: model.ReportFeeDetail{
				OrderDetailID:      999999,
				ProductID:          123456,
				CategoryID:         654321,
				ProductPriceAmount: 0.1,
				PromoAmount:        123456.789,
				FeeBaseAmount:      1e20,
				FeeFinalAmount:     1e-7,
			},
		},
	}

	var got bytes.Buffer
	b, err := newReportCSVBuilder(&got, constant.DefaultCSVWriteBufSize)
	if err != nil {
		t.Fatalf("newReportCSVBuilder: %v", err)
	}
	for _, rl := range rows {
		if err := b.appendRow(rl); err != nil {
			t.Fatalf("appendRow(%v): %v", rl, err)
		}
	}
	if err := b.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	var want bytes.Buffer
	cw := stdcsv.NewWriter(&want)
	if err := cw.Write(constant.ReportFileCSVHeaders); err != nil {
		t.Fatalf("csv.Write header: %v", err)
	}
	for _, rl := range rows {
		fields := strings.Split(strings.TrimSuffix(referenceCSVRow(rl), "\n"), ",")
		if err := cw.Write(fields); err != nil {
			t.Fatalf("csv.Write row: %v", err)
		}
	}
	cw.Flush()

	if got.String() != want.String() {
		t.Fatalf("mismatch:\n got %q\nwant %q", got.String(), want.String())
	}
}

// TestReportCSVFileBuilder_HeaderQuotingMatchesEncodingCSV verifies the quoting
// path (used by the header row) is byte-compatible with encoding/csv's default
// Writer across quoting edge cases.
func TestReportCSVFileBuilder_HeaderQuotingMatchesEncodingCSV(t *testing.T) {
	headers := [][]string{
		{"a", "b", "c"},
		{"", "", ""},
		{`a,b`, `c"d`, "e\nf", "g\rh"},
		{" leading", "trailing ", " x "},
		{`\.`},
	}

	for i, fields := range headers {
		var got bytes.Buffer
		bw := bufio.NewWriter(&got)
		b := &reportCSVFileBuilder{
			w:   zerocsv.NewWriter(bw),
			buf: bw,
		}
		if err := b.writeHeaderWith(fields); err != nil {
			t.Fatalf("case %d: writeHeaderWith: %v", i, err)
		}
		if err := b.flush(); err != nil {
			t.Fatalf("case %d: flush: %v", i, err)
		}

		var want bytes.Buffer
		cw := stdcsv.NewWriter(&want)
		if err := cw.Write(fields); err != nil {
			t.Fatalf("case %d: csv.Write: %v", i, err)
		}
		cw.Flush()

		if got.String() != want.String() {
			t.Errorf("case %d mismatch:\n got %q\nwant %q", i, got.String(), want.String())
		}
	}
}

// TestReportCSVFileBuilder_AppendRowReusesBuffer verifies appendRow reuses both
// its row slice and performs no heap allocation after the first call, keeping
// the steady state allocation-free.
func TestReportCSVFileBuilder_AppendRowReusesBuffer(t *testing.T) {
	b, err := newReportCSVBuilder(io.Discard, constant.DefaultCSVWriteBufSize)
	if err != nil {
		t.Fatalf("newReportCSVBuilder: %v", err)
	}
	rl := testReportLine(100, 200, 300)

	if err := b.appendRow(rl); err != nil {
		t.Fatalf("first appendRow: %v", err)
	}
	firstCap := cap(b.row)

	if err := b.appendRow(rl); err != nil {
		t.Fatalf("second appendRow: %v", err)
	}
	if cap(b.row) != firstCap {
		t.Fatalf("row slice grew across appendRow calls: %d -> %d", firstCap, cap(b.row))
	}

	allocs := testing.AllocsPerRun(100, func() {
		_ = b.appendRow(rl)
	})
	if allocs != 0 {
		t.Fatalf("appendRow allocated %v times/op (want 0)", allocs)
	}
}
