//go:build integration

package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/fikrimohammad/efficient-report-exporter/constant"
	apihandler "github.com/fikrimohammad/efficient-report-exporter/handler/api"
	mqhandler "github.com/fikrimohammad/efficient-report-exporter/handler/mq"
	apimodel "github.com/fikrimohammad/efficient-report-exporter/model/api"
	mqrepository "github.com/fikrimohammad/efficient-report-exporter/repository/mq"
	redisrepository "github.com/fikrimohammad/efficient-report-exporter/repository/redis"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	reportusecase "github.com/fikrimohammad/efficient-report-exporter/usecase/report"
	"github.com/fikrimohammad/go-dev-sdk/appinfo"
	commonredis "github.com/fikrimohammad/go-dev-sdk/redis"
	rocketmqconsumer "github.com/fikrimohammad/go-dev-sdk/rocketmq/consumer"
	rocketmqproducer "github.com/fikrimohammad/go-dev-sdk/rocketmq/producer"
)

// e2eBenchDetail mirrors the three fee details that seedBenchRows writes per
// report row, so the expected CSV rows can be reconstructed independently.
type e2eBenchDetail struct {
	categoryID         int64
	productID          int64
	productPriceAmount string
	promoAmount        string
	feeBaseAmount      string
	feeFinalAmount     string
}

var e2eBenchDetails = []e2eBenchDetail{
	{categoryID: 1, productID: 1, productPriceAmount: "9.99", promoAmount: "1", feeBaseAmount: "8.99", feeFinalAmount: "0.5"},
	{categoryID: 2, productID: 2, productPriceAmount: "19.99", promoAmount: "0", feeBaseAmount: "19.99", feeFinalAmount: "1.5"},
	{categoryID: 3, productID: 3, productPriceAmount: "29.99", promoAmount: "0", feeBaseAmount: "29.99", feeFinalAmount: "2"},
}

type e2eFlowConfig struct {
	shopID            int64
	rowCount          int
	maxSingleFileRows int
}

// TestEndToEndExportThroughRocketMQ exercises the single-file (CSV) export path
// end-to-end: POST /v1/reports/export publishes to RocketMQ, a real consumer
// runs the pipeline, and the downloaded CSV is verified against the seed data.
func TestEndToEndExportThroughRocketMQ(t *testing.T) {
	data, expected := runExportFlow(t, e2eFlowConfig{
		shopID:            424299,
		rowCount:          100,
		maxSingleFileRows: constant.DefaultMaxSingleFileRows,
	})
	verifyReportFile(t, data, expected)
}

// TestEndToEndExportThroughRocketMQZipped exercises the batched (ZIP) export
// path by capping max_single_file_rows below the seeded row count.
func TestEndToEndExportThroughRocketMQZipped(t *testing.T) {
	data, expected := runExportFlow(t, e2eFlowConfig{
		shopID:            424300,
		rowCount:          100,
		maxSingleFileRows: 10,
	})
	verifyReportFile(t, data, expected)
}

// runExportFlow drives the full asynchronous export through the HTTP API and
// the real queue, then downloads the produced report file. It returns the raw
// file bytes and the expected CSV data rows for the seeded report.
func runExportFlow(t *testing.T, cfg e2eFlowConfig) ([]byte, [][]string) {
	t.Helper()

	ctx := context.Background()
	d := setupDeps(t)

	redisClient, err := commonredis.New(d.infra.redis)
	if err != nil {
		t.Fatalf("init redis: %v", err)
	}
	t.Cleanup(func() { _ = redisClient.Close() })

	redisRepo, err := redisrepository.New(redisClient)
	if err != nil {
		t.Fatalf("redis repo: %v", err)
	}

	producerManager, err := rocketmqproducer.New(appinfo.Default())
	if err != nil {
		t.Fatalf("init mq producer: %v", err)
	}
	if err := producerManager.Register(rocketmqproducer.Config{
		Endpoints: d.infra.namesrv,
		Topic:     string(constant.MQTopicReporting),
	}); err != nil {
		t.Fatalf("register mq producer: %v", err)
	}
	if err := producerManager.Start(); err != nil {
		t.Fatalf("start mq producer: %v", err)
	}
	t.Cleanup(func() { _ = producerManager.Shutdown() })

	mqRepo, err := mqrepository.New(producerManager)
	if err != nil {
		t.Fatalf("mq repo: %v", err)
	}

	dl := newDynamicLoader(t, cfg.maxSingleFileRows, constant.DefaultMaxBatchPipelineWorkers)
	t.Cleanup(func() { _ = dl.Stop() })

	uc, err := reportusecase.New(d.mysqlRepo, mqRepo, redisRepo, d.s3Repo, dl)
	if err != nil {
		t.Fatalf("report use case: %v", err)
	}

	start := benchStart
	end := benchStart.Add(24 * time.Hour)
	if err := seedBenchRows(ctx, d.rawDB, cfg.shopID, cfg.rowCount, start, end); err != nil {
		t.Fatalf("seed report rows: %v", err)
	}

	// Wire the consumer before publishing, so the message is not missed while
	// the subscription is still being established.
	consumerCfg := d.infra.mq
	consumerCfg.Group = fmt.Sprintf("e2e_%d", time.Now().UnixNano())
	consumerCfg.ConsumeFromWhere = rocketmqconsumer.ConsumeFromLast

	mqHandler, err := mqhandler.New(uc)
	if err != nil {
		t.Fatalf("mq handler: %v", err)
	}

	consumerManager, err := rocketmqconsumer.New(appinfo.Default())
	if err != nil {
		t.Fatalf("init mq consumer: %v", err)
	}
	if err := consumerManager.Register(consumerCfg, mqHandler.ProcessExportReport); err != nil {
		t.Fatalf("register mq consumer: %v", err)
	}
	if err := consumerManager.Start(); err != nil {
		t.Fatalf("start mq consumer: %v", err)
	}
	t.Cleanup(func() { _ = consumerManager.Shutdown() })

	// Allow the consumer to finish rebalancing before publishing.
	time.Sleep(2 * time.Second)

	// Drive the flow through the real HTTP API.
	baseURL := "http://" + startAPIServer(t, uc)

	var exportResp apimodel.ExportReportResponse
	status := httpDoJSON(t, http.MethodPost, baseURL+constant.RouteExportReport, apimodel.ExportReportRequest{
		RequestID: strconv.FormatInt(time.Now().UnixNano(), 10),
		ShopID:    strconv.FormatInt(cfg.shopID, 10),
		StartTime: start.Format(time.RFC3339),
		EndTime:   end.Format(time.RFC3339),
	}, &exportResp)
	if status != http.StatusOK {
		t.Fatalf("POST export returned %d", status)
	}
	if exportResp.Data == nil || exportResp.Data.JobID == "" {
		t.Fatalf("expected a job_id in the response, got %+v", exportResp)
	}

	deadline := time.Now().Add(60 * time.Second)
	jobURL := fmt.Sprintf("%s%s/%s", baseURL, constant.RouteExportReport, exportResp.Data.JobID)
	var jobData apimodel.GetExportReportJobData
	for time.Now().Before(deadline) {
		var jobResp apimodel.GetExportReportJobResponse
		status := httpDoJSON(t, http.MethodGet, jobURL, nil, &jobResp)
		if status != http.StatusOK {
			t.Fatalf("GET job returned %d", status)
		}
		if jobResp.Data == nil {
			t.Fatalf("expected job data in the response, got %+v", jobResp)
		}
		jobData = *jobResp.Data
		if jobData.Status == string(constant.ExportReportJobStatusSuccess) || jobData.Status == string(constant.ExportReportJobStatusFailed) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if jobData.Status != string(constant.ExportReportJobStatusSuccess) {
		t.Fatalf("expected job success, got %s (err=%s)", jobData.Status, jobData.ErrorMessage)
	}
	if jobData.DownloadURL == "" {
		t.Fatal("expected a presigned download URL on success")
	}

	// Verify the list endpoint surfaces the completed job.
	var listResp apimodel.ListExportReportJobsResponse
	status = httpDoJSON(t, http.MethodGet, fmt.Sprintf("%s%s?shop_id=%d", baseURL, constant.RouteExportReportJobs, cfg.shopID), nil, &listResp)
	if status != http.StatusOK {
		t.Fatalf("GET jobs returned %d", status)
	}
	if listResp.Data == nil || len(listResp.Data.Jobs) == 0 {
		t.Fatalf("expected at least one job in the list, got %+v", listResp)
	}

	found := false
	for _, job := range listResp.Data.Jobs {
		if job.JobID == exportResp.Data.JobID {
			found = true
			if job.Status != string(constant.ExportReportJobStatusSuccess) {
				t.Fatalf("expected listed job status success, got %s", job.Status)
			}
		}
	}
	if !found {
		t.Fatalf("expected the created job %s in the list, got %+v", exportResp.Data.JobID, listResp.Data.Jobs)
	}

	return downloadFile(t, jobData.DownloadURL), expectedReportCSVRows(cfg.shopID, cfg.rowCount, start, end)
}

// startAPIServer starts a real Hertz server exposing the report export routes
// (as in app/api) on a random loopback port and returns its address.
func startAPIServer(t *testing.T, uc usecase.Report) string {
	t.Helper()

	h, err := apihandler.New(uc)
	if err != nil {
		t.Fatalf("api handler: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	engine := server.New(server.WithListener(ln), server.WithExitWaitTime(10*time.Millisecond))
	engine.POST(constant.RouteExportReport, h.RequestExportReport)
	engine.GET(constant.RouteExportReportJob, h.GetExportReportJob)
	engine.GET(constant.RouteExportReportJobs, h.ListExportReportJobs)

	go func() { _ = engine.Run() }()
	for i := 0; i < 100; i++ {
		if engine.IsRunning() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Cleanup(func() { _ = engine.Shutdown(context.Background()) })
	return ln.Addr().String()
}

// httpDoJSON performs an HTTP request, optionally JSON-encodes body, and
// decodes the JSON response into out (when non-nil). It returns the status code.
func httpDoJSON(t *testing.T, method, url string, body any, out any) int {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = strings.NewReader(string(raw))
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return resp.StatusCode
}

// downloadFile fetches the report file from its presigned URL.
func downloadFile(t *testing.T, url string) []byte {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("download report: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download report returned %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	return data
}

// expectedReportCSVRows reconstructs the CSV data rows that seedBenchRows
// produces for (shopID, n) rows across [start, end), in settlement-time order.
func expectedReportCSVRows(shopID int64, n int, start, end time.Time) [][]string {
	step := end.Sub(start) / time.Duration(n)
	rows := make([][]string, 0, n*benchDetailsPerRow)
	for i := 0; i < n; i++ {
		settle := start.Add(time.Duration(i) * step)
		pay := settle.Add(-time.Hour)
		create := pay.Add(-30 * time.Minute)
		for k, d := range e2eBenchDetails {
			rows = append(rows, []string{
				strconv.FormatInt(shopID, 10),
				strconv.FormatInt(int64(i+1), 10), // fee id
				strconv.FormatInt(int64(i+1), 10), // order id
				create.Format(constant.ReportLineTimeFormat),
				pay.Format(constant.ReportLineTimeFormat),
				settle.Format(constant.ReportLineTimeFormat),
				strconv.FormatInt(int64(i*benchDetailsPerRow+k+1), 10), // order detail id
				strconv.FormatInt(d.productID, 10),
				strconv.FormatInt(d.categoryID, 10),
				d.productPriceAmount,
				d.promoAmount,
				d.feeBaseAmount,
				d.feeFinalAmount,
			})
		}
	}
	return rows
}

// verifyReportFile checks that a downloaded report file (CSV or ZIP) has the
// expected header and data rows.
func verifyReportFile(t *testing.T, data []byte, expected [][]string) {
	t.Helper()

	headers, rows := parseReportFile(t, data)
	for _, h := range headers {
		if !slices.Equal(h, constant.ReportFileCSVHeaders) {
			t.Fatalf("unexpected CSV header:\n got  %v\n want %v", h, constant.ReportFileCSVHeaders)
		}
	}
	if len(rows) != len(expected) {
		t.Fatalf("expected %d data rows, got %d", len(expected), len(rows))
	}
	for i := range expected {
		if !slices.Equal(rows[i], expected[i]) {
			t.Fatalf("row %d mismatch:\n want %v\n got  %v", i, expected[i], rows[i])
		}
	}
}

// parseReportFile parses a report file into its header row(s) and data rows,
// handling both the single-file CSV and the batched ZIP (whose batch entries
// each carry their own header).
func parseReportFile(t *testing.T, data []byte) (headers [][]string, rows [][]string) {
	t.Helper()
	if isZip(data) {
		return parseZipReport(t, data)
	}
	h, r := parseCSVData(t, data)
	return [][]string{h}, r
}

func isZip(data []byte) bool {
	return len(data) >= 2 && data[0] == 'P' && data[1] == 'K'
}

func parseZipReport(t *testing.T, data []byte) ([][]string, [][]string) {
	t.Helper()

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	files := append([]*zip.File(nil), zr.File...)
	// Entry names encode the batch time range and sort lexicographically into
	// chronological order; the write order across concurrent batches is not
	// deterministic.
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	var headers [][]string
	var rows [][]string
	for _, f := range files {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s: %v", f.Name, err)
		}
		h, r := parseCSVData(t, b)
		headers = append(headers, h)
		rows = append(rows, r...)
	}
	return headers, rows
}

func parseCSVData(t *testing.T, data []byte) (header []string, rows [][]string) {
	t.Helper()

	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("empty csv file")
	}
	return records[0], records[1:]
}
