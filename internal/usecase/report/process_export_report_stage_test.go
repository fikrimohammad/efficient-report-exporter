package report

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/mock"
	"github.com/fikrimohammad/efficient-report-exporter/internal/model"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/fikrimohammad/go-dev-sdk/errgroup"
	"github.com/fikrimohammad/go-typedpipe/v2"
	"go.uber.org/mock/gomock"
)

func TestBuildReportFileName(t *testing.T) {
	if got := buildReportFileName(123, ".csv"); got != "report_123.csv" {
		t.Fatalf("unexpected csv name: %s", got)
	}
	if got := buildReportFileName(123, ".zip"); got != "report_123.zip" {
		t.Fatalf("unexpected zip name: %s", got)
	}
}

func TestBuildReportBatches_SlicesAndClampsLast(t *testing.T) {
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	re := &reportExporter{dynamicConfig: dl}

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Hour)

	batches := re.buildReportBatches(context.Background(), 100, start, end)

	if len(batches) != 3 {
		t.Fatalf("expected 3 batches over 5h at 2h size, got %d", len(batches))
	}
	if !batches[0].StartTime.Equal(start) || !batches[0].EndTime.Equal(start.Add(2*time.Hour)) {
		t.Fatal("first batch range incorrect")
	}
	if !batches[2].EndTime.Equal(end) {
		t.Fatal("last batch should be clamped to end")
	}
	for _, b := range batches {
		if b.ShopID != 100 {
			t.Fatal("shop id not propagated to batches")
		}
	}
}

func TestReportBatchEntryName(t *testing.T) {
	b := model.ReportBatch{
		StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC),
	}
	if got := b.EntryName(); got != "batch_20260101000000_20260101020000.csv" {
		t.Fatalf("unexpected entry name: %s", got)
	}
}

func TestAsyncZipReportBatchFiles_ZipsEntries(t *testing.T) {
	re := &reportExporter{}

	ctx := context.Background()
	eg := errgroup.New(ctx)

	writer, reader := typedpipe.New[model.ReportBatchFile]()

	zipReader, err := re.asyncZipReportBatchFiles(eg, reader)
	if err != nil {
		t.Fatalf("asyncZipReportBatchFiles failed: %v", err)
	}

	b1, err := re.compressReportCSVFile("batch_1.csv", io.NopCloser(strings.NewReader("a,b\n1,2\n")))
	if err != nil {
		t.Fatalf("compress batch 1: %v", err)
	}
	b2, err := re.compressReportCSVFile("batch_2.csv", io.NopCloser(strings.NewReader("a,b\n3,4\n")))
	if err != nil {
		t.Fatalf("compress batch 2: %v", err)
	}

	if err := writer.Write(ctx, b1); err != nil {
		t.Fatalf("write batch 1: %v", err)
	}
	if err := writer.Write(ctx, b2); err != nil {
		t.Fatalf("write batch 2: %v", err)
	}
	writer.Close()

	var zipBuf bytes.Buffer
	if _, err := io.Copy(&zipBuf, zipReader); err != nil {
		t.Fatalf("read zip: %v", err)
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBuf.Bytes()), int64(zipBuf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("expected 2 zip entries, got %d", len(zr.File))
	}
	if zr.File[0].Name != "batch_1.csv" || zr.File[1].Name != "batch_2.csv" {
		t.Fatalf("unexpected entry names: %q, %q", zr.File[0].Name, zr.File[1].Name)
	}

	// The entries must round-trip through deflate + CreateRaw back to the
	// original CSV bytes.
	for i, want := range []string{"a,b\n1,2\n", "a,b\n3,4\n"} {
		rc, err := zr.File[i].Open()
		if err != nil {
			t.Fatalf("open entry %d: %v", i, err)
		}
		got, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read entry %d: %v", i, err)
		}
		if string(got) != want {
			t.Fatalf("entry %d content mismatch: got %q want %q", i, got, want)
		}
	}
}

func TestRunExportReportPipeline_BatchedPathProducesZip(t *testing.T) {
	ctrl := gomock.NewController(t)
	mysql := mock.NewMockMySQL(ctrl)
	s3 := mock.NewMockS3(ctrl)
	mq := mock.NewMockMQ(ctrl)

	mysql.EXPECT().
		CountReport(gomock.Any(), gomock.Any()).
		Return(int64(100001), nil).
		AnyTimes()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Hour) // 3 batches at 2h

	mysql.EXPECT().
		QueryReport(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, filter repository.QueryReportFilter) ([]*model.Report, error) {
			return []*model.Report{{
				ID:                  1,
				ShopID:              100,
				OrderID:             1,
				OrderSettlementTime: (*filter.OrderSettlementTimeRange.StartTime).UnixMilli(),
				Details:             []byte(`[{"order_detail_id":1,"product_id":1,"fee_final_amount":1.5}]`),
			}}, nil
		}).
		AnyTimes()

	var (
		uploadedName string
		uploadedData bytes.Buffer
	)
	s3.EXPECT().
		UploadReportFile(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, params repository.UploadReportFileParams) error {
			uploadedName = params.FileName
			_, err := io.Copy(&uploadedData, params.FileData)
			return err
		}).
		AnyTimes()

	var updatedStatus constant.ExportReportJobStatus
	mysql.EXPECT().
		UpdateExportReportJob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, params repository.UpdateExportReportJobParams) error {
			updatedStatus = params.Status
			return nil
		}).
		AnyTimes()

	mq.EXPECT().PublishExportReportDoneMsg(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	re := &reportExporter{
		mysqlRepository: mysql,
		mqRepository:    mq,
		s3Repository:    s3,
		dynamicConfig:   dl,
	}

	err := re.runExportReportPipeline(context.Background(), 100, start, end, 99)
	if err != nil {
		t.Fatalf("runExportReportPipeline failed: %v", err)
	}

	if updatedStatus != constant.ExportReportJobStatusSuccess {
		t.Fatalf("expected job marked success, got %s", updatedStatus)
	}
	if !strings.HasSuffix(uploadedName, ".zip") {
		t.Fatalf("expected .zip upload, got %s", uploadedName)
	}

	zr, err := zip.NewReader(bytes.NewReader(uploadedData.Bytes()), int64(uploadedData.Len()))
	if err != nil {
		t.Fatalf("open uploaded zip: %v", err)
	}
	if len(zr.File) != 3 {
		t.Fatalf("expected 3 zip entries, got %d", len(zr.File))
	}
}

// TestAsyncBuildReportCSVFile_ReusedRowBufferNoDuplication writes many distinct
// rows through the CSV stage and verifies the output is exactly header + N rows
// in order — guarding against the buffer-reuse bug where reusing rowBuf across
// rows could leak the previous row's bytes into the next one.
func TestAsyncBuildReportCSVFile_ReusedRowBufferNoDuplication(t *testing.T) {
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	re := &reportExporter{dynamicConfig: dl}

	ctx := context.Background()
	eg := errgroup.New(ctx)

	lineWriter, lineReader := typedpipe.New[model.ReportLine]()

	csvReader, err := re.asyncBuildReportCSVFile(eg, lineReader)
	if err != nil {
		t.Fatalf("asyncBuildReportCSVFile: %v", err)
	}

	const n = 50
	for i := 0; i < n; i++ {
		if err := lineWriter.Write(ctx, model.ReportLine{
			ShopID:  int64(i + 1),
			FeeID:   int64(i + 1),
			OrderID: int64(i + 1),
			ReportFeeDetail: model.ReportFeeDetail{
				OrderDetailID:  int64(i + 1),
				ProductID:      int64(i + 1),
				CategoryID:     int64(i + 1),
				FeeFinalAmount: float64(i + 1),
			},
		}); err != nil {
			t.Fatalf("write line %d: %v", i, err)
		}
	}
	lineWriter.Close()

	data, err := io.ReadAll(csvReader)
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if err := eg.Wait(); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != n+1 {
		t.Fatalf("expected %d records (header + %d rows), got %d", n+1, n, len(records))
	}
	if !strings.EqualFold(records[0][0], "Shop ID") {
		t.Fatalf("unexpected header first column: %q", records[0][0])
	}
	for i, rec := range records[1:] {
		want := strconv.Itoa(i + 1)
		if rec[0] != want {
			t.Fatalf("row %d shop_id = %q, want %q (duplication or ordering bug)", i+1, rec[0], want)
		}
	}
}
