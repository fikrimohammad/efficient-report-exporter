package report

import (
	"bufio"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/model"
)

// reportCSVFileBuilder writes the report export CSV file: a header row written
// once at construction, then one data row per appendRow call. It wraps the
// caller's *bufio.Writer and reuses its internal buffer, so the steady-state
// appendRow loop is allocation-free.
type reportCSVFileBuilder struct {
	w   *bufio.Writer
	buf []byte
}

// newReportCSVBuilder returns a builder writing to buf and writes the report
// CSV header row to the stream.
func newReportCSVBuilder(buf *bufio.Writer) (*reportCSVFileBuilder, error) {
	b := &reportCSVFileBuilder{w: buf}
	if err := b.writeHeader(); err != nil {
		return nil, err
	}
	return b, nil
}

// appendRow writes rl as one CSV record followed by a newline.
func (b *reportCSVFileBuilder) appendRow(rl model.ReportLine) error {
	b.buf = rl.MarshalCSV(b.buf[:0])
	b.buf = append(b.buf, '\n')
	_, err := b.w.Write(b.buf)
	return err
}

// writeHeader writes the report column headers as a quoted CSV record.
func (b *reportCSVFileBuilder) writeHeader() error {
	b.buf = b.appendRecord(b.buf[:0], constant.ReportFileCSVHeaders)
	b.buf = append(b.buf, '\n')
	_, err := b.w.Write(b.buf)
	return err
}

// appendRecord appends fields to dst as one comma-separated record.
func (b *reportCSVFileBuilder) appendRecord(dst []byte, fields []string) []byte {
	for i, f := range fields {
		if i > 0 {
			dst = append(dst, ',')
		}
		dst = b.appendField(dst, f)
	}
	return dst
}

// appendField appends field to dst, quoting it when required.
func (b *reportCSVFileBuilder) appendField(dst []byte, field string) []byte {
	if !b.fieldNeedsQuotes(field) {
		return append(dst, field...)
	}

	dst = append(dst, '"')
	for i := 0; i < len(field); i++ {
		if field[i] == '"' {
			dst = append(dst, '"', '"')
		} else {
			dst = append(dst, field[i])
		}
	}
	return append(dst, '"')
}

// fieldNeedsQuotes mirrors encoding/csv's fieldNeedsQuotes for the default
// comma separator.
func (b *reportCSVFileBuilder) fieldNeedsQuotes(field string) bool {
	if field == "" {
		return false
	}
	if field == `\.` {
		return true
	}
	if strings.ContainsRune(field, ',') {
		return true
	}
	if strings.ContainsAny(field, "\"\r\n") {
		return true
	}
	r1, _ := utf8.DecodeRuneInString(field)
	return unicode.IsSpace(r1)
}
