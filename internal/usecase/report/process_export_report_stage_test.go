package report

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
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

	pr1, pw1 := io.Pipe()
	go func() { _, _ = pw1.Write([]byte("a,b\n1,2\n")); _ = pw1.Close() }()
	pr2, pw2 := io.Pipe()
	go func() { _, _ = pw2.Write([]byte("a,b\n3,4\n")); _ = pw2.Close() }()

	if err := writer.Write(ctx, model.ReportBatchFile{Name: "batch_1.csv", Reader: pr1}); err != nil {
		t.Fatalf("write batch 1: %v", err)
	}
	if err := writer.Write(ctx, model.ReportBatchFile{Name: "batch_2.csv", Reader: pr2}); err != nil {
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
				OrderSettlementTime: *filter.OrderSettlementTimeRange.StartTime,
				Details:             model.ReportFeeDetails{{OrderDetailID: 1, ProductID: 1, FeeFinalAmount: 1.5}},
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
