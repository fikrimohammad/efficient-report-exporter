package report

import (
	"bufio"
	"bytes"
	stdcsv "encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/model"
)

func testReportLine(shopID, feeID, orderID int64) model.ReportLine {
	return model.ReportLine{
		ShopID:              shopID,
		OrderID:             orderID,
		OrderCreationTime:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		OrderPaymentTime:    time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC),
		OrderSettlementTime: time.Date(2026, 1, 2, 3, 4, 7, 0, time.UTC),
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

// TestReportCSVFileBuilder_MatchesEncodingCSV proves the builder's output is
// byte-identical to encoding/csv's default Writer for the real header row and
// data rows.
func TestReportCSVFileBuilder_MatchesEncodingCSV(t *testing.T) {
	rows := []model.ReportLine{
		testReportLine(100, 200, 300),
		testReportLine(100, 201, 301),
		testReportLine(101, 202, 302),
	}

	var got bytes.Buffer
	bw := bufio.NewWriter(&got)
	b, err := newReportCSVBuilder(bw)
	if err != nil {
		t.Fatalf("newReportCSVBuilder: %v", err)
	}
	for _, rl := range rows {
		if err := b.appendRow(rl); err != nil {
			t.Fatalf("appendRow(%v): %v", rl, err)
		}
	}
	if err := bw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var want bytes.Buffer
	cw := stdcsv.NewWriter(&want)
	if err := cw.Write(constant.ReportFileCSVHeaders); err != nil {
		t.Fatalf("csv.Write header: %v", err)
	}
	for _, rl := range rows {
		fields := strings.Split(string(rl.MarshalCSV(nil)), ",")
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
		b := reportCSVFileBuilder{w: bufio.NewWriter(&got)}
		if err := b.writeHeaderWith(fields); err != nil {
			t.Fatalf("case %d: writeHeaderWith: %v", i, err)
		}
		if err := b.w.Flush(); err != nil {
			t.Fatalf("case %d: Flush: %v", i, err)
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

// TestReportCSVFileBuilder_AppendRowReusesBuffer verifies appendRow reuses its
// buffer after the first call, keeping the steady state allocation-free.
func TestReportCSVFileBuilder_AppendRowReusesBuffer(t *testing.T) {
	b := &reportCSVFileBuilder{w: bufio.NewWriter(&bytes.Buffer{})}
	rl := testReportLine(100, 200, 300)

	if err := b.appendRow(rl); err != nil {
		t.Fatalf("first appendRow: %v", err)
	}
	firstCap := cap(b.buf)

	if err := b.appendRow(rl); err != nil {
		t.Fatalf("second appendRow: %v", err)
	}
	if cap(b.buf) != firstCap {
		t.Fatalf("buffer grew across appendRow calls: %d -> %d", firstCap, cap(b.buf))
	}
}

// writeHeaderWith writes fields as a quoted CSV record followed by a newline,
// exercising the same quoting path as writeHeader on arbitrary fields.
func (b *reportCSVFileBuilder) writeHeaderWith(fields []string) error {
	b.buf = b.appendRecord(b.buf[:0], fields)
	b.buf = append(b.buf, '\n')
	_, err := b.w.Write(b.buf)
	return err
}
