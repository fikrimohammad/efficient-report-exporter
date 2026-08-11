package report

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-dev-sdk/errgroup"
	"github.com/fikrimohammad/go-typedpipe/v2"
)

// ---------------------------------------------------------------------------
// ProcessExportReport (top-level)
// ---------------------------------------------------------------------------

func TestProcessExportReport_FullPipelineSuccess(t *testing.T) {
	mysql := defaultMockMySQL()
	mq := defaultMockMQ()
	redis := defaultMockRedis()
	s3 := defaultMockS3()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	reports := []*model.Report{
		{
			ID: 1, ShopID: 100, OrderID: 1,
			OrderCreationTime:   time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
			OrderPaymentTime:    time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC),
			OrderSettlementTime: time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC),
			FeeID:               10,
			Details: model.ReportFeeDetails{
				{OrderDetailID: 1, CategoryID: 100, ProductID: 1000, ProductPriceAmount: 10.5, PromoAmount: 1.0, FeeBaseAmount: 9.5, FeeFinalAmount: 9.0},
				{OrderDetailID: 2, CategoryID: 200, ProductID: 2000, ProductPriceAmount: 20.0, PromoAmount: 2.0, FeeBaseAmount: 18.0, FeeFinalAmount: 17.0},
			},
		},
		{
			ID: 2, ShopID: 100, OrderID: 2,
			OrderCreationTime:   time.Date(2025, 1, 1, 14, 0, 0, 0, time.UTC),
			OrderPaymentTime:    time.Date(2025, 1, 1, 15, 0, 0, 0, time.UTC),
			OrderSettlementTime: time.Date(2025, 1, 2, 16, 0, 0, 0, time.UTC),
			FeeID:               11,
			Details: model.ReportFeeDetails{
				{OrderDetailID: 3, CategoryID: 300, ProductID: 3000, ProductPriceAmount: 30.0, PromoAmount: 3.0, FeeBaseAmount: 27.0, FeeFinalAmount: 26.0},
			},
		},
	}

	queryReportPage := 0
	mysql.queryReportFn = func(_ context.Context, filter repository.QueryReportFilter) ([]*model.Report, error) {
		if queryReportPage >= 2 {
			return nil, nil
		}
		page := reports[queryReportPage : queryReportPage+1]
		queryReportPage++
		return page, nil
	}

	var uploadedFile struct {
		mu   sync.Mutex
		name string
		data bytes.Buffer
	}
	s3.uploadReportFn = func(_ context.Context, params repository.UploadReportFileParams) error {
		uploadedFile.mu.Lock()
		defer uploadedFile.mu.Unlock()
		uploadedFile.name = params.FileName
		_, err := io.Copy(&uploadedFile.data, params.FileData)
		return err
	}

	mysql.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return []*model.ExportReportJob{{
			ID:     99,
			ShopID: 100,
			Status: constant.ExportReportJobStatusProcessing,
		}}, nil
	}

	mysql.updateExportReportJob = func(_ context.Context, params repository.UpdateExportReportJobParams) error {
		if params.Status != constant.ExportReportJobStatusSuccess {
			t.Errorf("expected job to be marked success, got %s", params.Status)
		}
		return nil
	}

	mu := &useCase{
		mySQLRepository: mysql,
		mqRepository:    mq,
		redisRepository: redis,
		s3Repository:    s3,
		dynamicConfig:   dl,
	}

	err := mu.ProcessExportReport(context.Background(), usecase.ProcessExportReportParams{JobID: 99})
	if err != nil {
		t.Fatalf("ProcessExportReport failed: %v", err)
	}

	uploadedFile.mu.Lock()
	csvData := uploadedFile.data.String()
	_ = uploadedFile.name
	uploadedFile.mu.Unlock()

	if !strings.Contains(csvData, "100") {
		t.Fatal("CSV should contain ShopID 100")
	}
	if !strings.Contains(csvData, "10.5") {
		t.Fatal("CSV should contain product price 10.5")
	}
	if !strings.Contains(csvData, constant.ReportFileCSVHeaders[0]) {
		t.Fatal("CSV should contain headers")
	}
}

func TestProcessExportReport_JobAlreadySuccess(t *testing.T) {
	mysql := defaultMockMySQL()
	redis := defaultMockRedis()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mysql.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return []*model.ExportReportJob{{
			ID:     99,
			Status: constant.ExportReportJobStatusSuccess,
		}}, nil
	}

	mu := &useCase{
		mySQLRepository: mysql,
		redisRepository: redis,
		dynamicConfig:   dl,
	}

	var s3Called bool
	mu.s3Repository = &mockS3{
		uploadReportFn: func(_ context.Context, _ repository.UploadReportFileParams) error {
			s3Called = true
			return nil
		},
	}

	err := mu.ProcessExportReport(context.Background(), usecase.ProcessExportReportParams{JobID: 99})
	if err != nil {
		t.Fatalf("expected no error for already-successful job, got: %v", err)
	}
	if s3Called {
		t.Fatal("S3 should not be called for already-successful job")
	}
}

func TestProcessExportReport_LockError(t *testing.T) {
	redis := defaultMockRedis()
	redis.lockProcessFn = func(_ context.Context, _ repository.LockExportReportProcess) error {
		return errors.New("lock failed")
	}

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mu := &useCase{
		redisRepository: redis,
		dynamicConfig:   dl,
	}

	err := mu.ProcessExportReport(context.Background(), usecase.ProcessExportReportParams{JobID: 99})
	if err == nil || err.Error() != "lock failed" {
		t.Fatalf("expected lock error, got: %v", err)
	}
}

func TestProcessExportReport_JobNotFound(t *testing.T) {
	mysql := defaultMockMySQL()
	redis := defaultMockRedis()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mysql.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return nil, nil
	}

	mu := &useCase{
		mySQLRepository: mysql,
		redisRepository: redis,
		dynamicConfig:   dl,
	}

	err := mu.ProcessExportReport(context.Background(), usecase.ProcessExportReportParams{JobID: 99})
	if err == nil || !strings.Contains(err.Error(), "job not found") {
		t.Fatalf("expected 'job not found' error, got: %v", err)
	}
}

func TestProcessExportReport_JobQueryError(t *testing.T) {
	mysql := defaultMockMySQL()
	redis := defaultMockRedis()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mysql.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return nil, errors.New("query failed")
	}

	mu := &useCase{
		mySQLRepository: mysql,
		redisRepository: redis,
		dynamicConfig:   dl,
	}

	err := mu.ProcessExportReport(context.Background(), usecase.ProcessExportReportParams{JobID: 99})
	if err == nil || err.Error() != "query failed" {
		t.Fatalf("expected 'query failed' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// asyncFetchReports
// ---------------------------------------------------------------------------

func TestAsyncFetchReports_SinglePage(t *testing.T) {
	mysql := defaultMockMySQL()
	mysql.queryReportFn = func(_ context.Context, _ repository.QueryReportFilter) ([]*model.Report, error) {
		return []*model.Report{
			{ID: 1, ShopID: 100},
			{ID: 2, ShopID: 100},
		}, nil
	}

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	re := &reportExporter{
		mysqlRepository: mysql,
		dynamicConfig:   dl,
	}

	ctx := context.Background()
	eg := errgroup.New(ctx)

	reader, err := re.asyncFetchReports(eg, 100, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("asyncFetchReports failed: %v", err)
	}

	var got []model.Report
	for {
		r, readErr := reader.Read(ctx)
		if errors.Is(readErr, typedpipe.ErrPipeClosed) {
			break
		}
		if readErr != nil {
			t.Fatalf("read error: %v", readErr)
		}
		got = append(got, r)
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(got))
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Fatal("reports received in wrong order")
	}
}

func TestAsyncFetchReports_MultiplePages(t *testing.T) {
	mysql := defaultMockMySQL()
	callCount := 0
	mysql.queryReportFn = func(_ context.Context, filter repository.QueryReportFilter) ([]*model.Report, error) {
		callCount++
		if callCount == 1 {
			return []*model.Report{{ID: 1}, {ID: 2}}, nil
		}
		if callCount == 2 {
			return []*model.Report{{ID: 3}}, nil
		}
		return nil, nil
	}

	dl := newTestDynamicLoader(t, 2)
	defer func() { _ = dl.Stop() }()

	re := &reportExporter{
		mysqlRepository: mysql,
		dynamicConfig:   dl,
	}

	ctx := context.Background()
	eg := errgroup.New(ctx)

	reader, err := re.asyncFetchReports(eg, 100, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("asyncFetchReports failed: %v", err)
	}

	var got []model.Report
	for {
		r, readErr := reader.Read(ctx)
		if errors.Is(readErr, typedpipe.ErrPipeClosed) {
			break
		}
		if readErr != nil {
			t.Fatalf("read error: %v", readErr)
		}
		got = append(got, r)
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 reports across pages, got %d", len(got))
	}
	if callCount != 2 {
		t.Fatalf("expected 2 query calls (first returns 2 = limit, second returns 1 < limit), got %d", callCount)
	}
}

func TestAsyncFetchReports_QueryError(t *testing.T) {
	mysql := defaultMockMySQL()
	mysql.queryReportFn = func(_ context.Context, _ repository.QueryReportFilter) ([]*model.Report, error) {
		return nil, errors.New("query error")
	}

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	re := &reportExporter{
		mysqlRepository: mysql,
		dynamicConfig:   dl,
	}

	ctx := context.Background()
	eg := errgroup.New(ctx)

	reader, err := re.asyncFetchReports(eg, 100, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("asyncFetchReports should not return error directly: %v", err)
	}

	_, readErr := reader.Read(ctx)
	if readErr == nil || !strings.Contains(readErr.Error(), "query error") {
		t.Fatalf("expected 'query error' propagated, got: %v", readErr)
	}

	_ = eg.Wait() // error already propagated
}

// ---------------------------------------------------------------------------
// asyncBuildReportLine
// ---------------------------------------------------------------------------

func TestAsyncBuildReportLine_FlattensDetails(t *testing.T) {
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	re := &reportExporter{
		dynamicConfig: dl,
	}

	ctx := context.Background()
	eg := errgroup.New(ctx)

	reportWriter, reportReader := typedpipe.New[model.Report]()

	lineReader, err := re.asyncBuildReportLine(ctx, eg, reportReader)
	if err != nil {
		t.Fatalf("asyncBuildReportLine failed: %v", err)
	}

	err = reportWriter.Write(ctx, model.Report{
		ID: 1, ShopID: 100, OrderID: 1001, FeeID: 10,
		OrderSettlementTime: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		Details: model.ReportFeeDetails{
			{OrderDetailID: 1, ProductID: 1, FeeFinalAmount: 10.0},
			{OrderDetailID: 2, ProductID: 2, FeeFinalAmount: 20.0},
		},
	})
	if err != nil {
		t.Fatalf("write report: %v", err)
	}
	reportWriter.Close()

	var lines []model.ReportLine
	for {
		line, readErr := lineReader.Read(ctx)
		if errors.Is(readErr, typedpipe.ErrPipeClosed) {
			break
		}
		if readErr != nil {
			t.Fatalf("read line: %v", readErr)
		}
		lines = append(lines, line)
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 report lines (one per detail), got %d", len(lines))
	}
	if lines[0].OrderDetailID != 1 || lines[1].OrderDetailID != 2 {
		t.Fatal("report lines not in correct order")
	}
	if lines[0].ShopID != 100 || lines[0].FeeID != 10 {
		t.Fatal("parent fields not propagated to report lines")
	}
}

// ---------------------------------------------------------------------------
// asyncBuildReportCSVFile
// ---------------------------------------------------------------------------

func TestAsyncBuildReportCSVFile_WritesHeadersAndRows(t *testing.T) {
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	re := &reportExporter{
		dynamicConfig: dl,
	}

	ctx := context.Background()
	eg := errgroup.New(ctx)

	lineWriter, lineReader := typedpipe.New[model.ReportLine]()

	csvReader, err := re.asyncBuildReportCSVFile(ctx, eg, lineReader)
	if err != nil {
		t.Fatalf("asyncBuildReportCSVFile failed: %v", err)
	}

	_ = lineWriter.Write(ctx, model.ReportLine{
		ShopID: 100, OrderID: 1001, FeeID: 10,
		OrderSettlementTime: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		ReportFeeDetail: model.ReportFeeDetail{
			OrderDetailID: 1, ProductID: 100, FeeFinalAmount: 15.5,
		},
	})
	lineWriter.Close()

	var csvBuf bytes.Buffer
	_, copyErr := io.Copy(&csvBuf, csvReader)
	if copyErr != nil {
		t.Fatalf("read csv: %v", copyErr)
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	output := csvBuf.String()
	if !strings.Contains(output, constant.ReportFileCSVHeaders[0]) {
		t.Fatal("CSV output missing headers")
	}
	if !strings.Contains(output, "15.5") {
		t.Fatal("CSV output missing fee final amount")
	}
}

func TestAsyncBuildReportCSVFile_PipeCloseOnLineReaderClose(t *testing.T) {
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	re := &reportExporter{
		dynamicConfig: dl,
	}

	ctx := context.Background()
	eg := errgroup.New(ctx)

	lineWriter, lineReader := typedpipe.New[model.ReportLine]()

	csvReader, err := re.asyncBuildReportCSVFile(ctx, eg, lineReader)
	if err != nil {
		t.Fatalf("asyncBuildReportCSVFile failed: %v", err)
	}

	lineWriter.Close()

	_, readErr := io.ReadAll(csvReader)
	if readErr != nil {
		t.Fatalf("unexpected error after closing line reader: %v", readErr)
	}
	if err := eg.Wait(); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// asyncUploadReportFile
// ---------------------------------------------------------------------------

func TestAsyncUploadReportFile_Success(t *testing.T) {
	s3 := defaultMockS3()

	var uploaded repository.UploadReportFileParams
	s3.uploadReportFn = func(_ context.Context, params repository.UploadReportFileParams) error {
		uploaded = params
		return nil
	}

	re := &reportExporter{
		s3Repository: s3,
	}

	ctx := context.Background()
	eg := errgroup.New(ctx)

	pipeR, pipeW := io.Pipe()
	go func() {
		_, _ = pipeW.Write([]byte("test,csv,data\n"))
		_ = pipeW.Close()
	}()

	fileName, err := re.asyncUploadReportFile(eg, pipeR)
	if err != nil {
		t.Fatalf("asyncUploadReportFile failed: %v", err)
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	if fileName == "" {
		t.Fatal("expected non-empty file name")
	}
	if !strings.HasSuffix(fileName, ".csv") {
		t.Fatalf("expected .csv extension, got %s", fileName)
	}
	if uploaded.FileName != fileName {
		t.Fatalf("expected upload filename %s, got %s", fileName, uploaded.FileName)
	}
}

func TestAsyncUploadReportFile_S3Error(t *testing.T) {
	s3 := defaultMockS3()
	s3.uploadReportFn = func(_ context.Context, _ repository.UploadReportFileParams) error {
		return errors.New("s3 upload error")
	}

	re := &reportExporter{
		s3Repository: s3,
	}

	ctx := context.Background()
	eg := errgroup.New(ctx)

	pipeR, _ := io.Pipe()

	_, err := re.asyncUploadReportFile(eg, pipeR)
	if err != nil {
		t.Fatalf("asyncUploadReportFile should not return error directly: %v", err)
	}

	waitErr := eg.Wait()
	if waitErr == nil || !strings.Contains(waitErr.Error(), "s3 upload error") {
		t.Fatalf("expected 's3 upload error', got: %v", waitErr)
	}
}

// ---------------------------------------------------------------------------
// runExportReportPipeline integration
// ---------------------------------------------------------------------------

func TestRunExportReportPipeline_MarksJobFailedOnError(t *testing.T) {
	mysql := defaultMockMySQL()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mysql.queryReportFn = func(_ context.Context, _ repository.QueryReportFilter) ([]*model.Report, error) {
		return nil, errors.New("fetch error")
	}

	var updatedStatus constant.ExportReportJobStatus
	var updatedExtra model.ExportReportJobExtra
	mysql.updateExportReportJob = func(_ context.Context, params repository.UpdateExportReportJobParams) error {
		updatedStatus = params.Status
		updatedExtra = params.Extra
		return nil
	}

	mysql.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return []*model.ExportReportJob{{ID: 99, Status: constant.ExportReportJobStatusProcessing}}, nil
	}

	re := &reportExporter{
		mysqlRepository: mysql,
		dynamicConfig:   dl,
	}

	err := re.runExportReportPipeline(context.Background(), 100, time.Time{}, time.Time{}, 99)
	if err == nil {
		t.Fatal("expected error from failed pipeline")
	}

	if updatedStatus != constant.ExportReportJobStatusFailed {
		t.Fatalf("expected job marked as failed, got %s", updatedStatus)
	}
	if updatedExtra.ErrMsg == nil || *updatedExtra.ErrMsg == "" {
		t.Fatalf("expected non-empty error message, got nil or empty")
	}
}

func TestExportReportCSVWithDefaults(t *testing.T) {
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	re := &reportExporter{
		dynamicConfig: dl,
	}

	ctx := context.Background()
	eg := errgroup.New(ctx)

	lineWriter, lineReader := typedpipe.New[model.ReportLine]()

	csvReader, err := re.asyncBuildReportCSVFile(ctx, eg, lineReader)
	if err != nil {
		t.Fatalf("asyncBuildReportCSVFile: %v", err)
	}

	for i := 0; i < 100; i++ {
		_ = lineWriter.Write(ctx, model.ReportLine{
			ShopID: 1, OrderID: int64(i), FeeID: 10,
			ReportFeeDetail: model.ReportFeeDetail{ProductID: int64(i), FeeFinalAmount: float64(i)},
		})
	}
	lineWriter.Close()

	_, copyErr := io.ReadAll(csvReader)
	if copyErr != nil {
		t.Fatalf("read csv: %v", copyErr)
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("pipeline error: %v", err)
	}
}
