package report

import (
	"bufio"
	"io"
	"time"

	zerocsv "github.com/fikrimohammad/go-zerocsv"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/model"
)

// reportCSVFileBuilder writes the report export CSV file: a header row written
// once at construction, then one data row per appendRow call. It wraps a
// *zerocsv.Writer over a caller-sized *bufio.Writer and reuses one []Column
// row, so the steady-state appendRow loop is allocation-free.
type reportCSVFileBuilder struct {
	w   *zerocsv.Writer
	buf *bufio.Writer
	row []zerocsv.Column
}

// newReportCSVBuilder returns a builder writing to w and writes the report CSV
// header row to the stream. The builder owns a buffered writer of bufSize bytes
// and must be flushed via flush before w is closed.
func newReportCSVBuilder(w io.Writer, bufSize int) (*reportCSVFileBuilder, error) {
	bw := bufio.NewWriterSize(w, bufSize)
	b := &reportCSVFileBuilder{
		w:   zerocsv.NewWriter(bw),
		buf: bw,
		row: make([]zerocsv.Column, len(constant.ReportFileCSVHeaders)),
	}
	if err := b.writeHeader(); err != nil {
		return nil, err
	}
	return b, nil
}

// flush pushes any buffered CSV bytes to the underlying writer.
func (b *reportCSVFileBuilder) flush() error {
	if err := b.w.Flush(); err != nil {
		return err
	}
	return b.buf.Flush()
}

// appendRow writes rl as one CSV record followed by a newline.
func (b *reportCSVFileBuilder) appendRow(rl model.ReportLine) error {
	row := b.row
	row[0] = zerocsv.ColumnInt64(rl.ShopID)
	row[1] = zerocsv.ColumnInt64(rl.FeeID)
	row[2] = zerocsv.ColumnInt64(rl.OrderID)
	row[3] = zerocsv.ColumnTime(time.UnixMilli(rl.OrderCreationTime).UTC(), constant.ReportLineTimeFormat)
	row[4] = zerocsv.ColumnTime(time.UnixMilli(rl.OrderPaymentTime).UTC(), constant.ReportLineTimeFormat)
	row[5] = zerocsv.ColumnTime(time.UnixMilli(rl.OrderSettlementTime).UTC(), constant.ReportLineTimeFormat)
	row[6] = zerocsv.ColumnInt64(rl.OrderDetailID)
	row[7] = zerocsv.ColumnInt64(rl.ProductID)
	row[8] = zerocsv.ColumnInt64(rl.CategoryID)
	row[9] = zerocsv.ColumnFloat64(rl.ProductPriceAmount)
	row[10] = zerocsv.ColumnFloat64(rl.PromoAmount)
	row[11] = zerocsv.ColumnFloat64(rl.FeeBaseAmount)
	row[12] = zerocsv.ColumnFloat64(rl.FeeFinalAmount)
	return b.w.Write(row...)
}

// writeHeader writes the report column headers as one CSV record.
func (b *reportCSVFileBuilder) writeHeader() error {
	headers := constant.ReportFileCSVHeaders
	row := b.row[:0]
	for i := range headers {
		row = append(row, zerocsv.ColumnString(headers[i]))
	}
	return b.w.Write(row...)
}
