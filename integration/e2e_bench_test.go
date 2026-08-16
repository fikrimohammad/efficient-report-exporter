//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	mqhandler "github.com/fikrimohammad/efficient-report-exporter/internal/handler/mq"
	apimodel "github.com/fikrimohammad/efficient-report-exporter/internal/model/api"
	mqrepository "github.com/fikrimohammad/efficient-report-exporter/internal/repository/mq"
	redisrepository "github.com/fikrimohammad/efficient-report-exporter/internal/repository/redis"
	reportusecase "github.com/fikrimohammad/efficient-report-exporter/internal/usecase/report"
	"github.com/fikrimohammad/go-dev-sdk/appinfo"
	commonredis "github.com/fikrimohammad/go-dev-sdk/redis"
	rocketmqconsumer "github.com/fikrimohammad/go-dev-sdk/rocketmq/consumer"
	rocketmqproducer "github.com/fikrimohammad/go-dev-sdk/rocketmq/producer"
)

// BenchmarkE2ESingle measures the client-perceived end-to-end latency of the
// single-CSV export path: POST /v1/reports/export → RocketMQ → consumer → poll
// until success → download. Unlike BenchmarkRealSingle (which calls the use case
// directly), this drives the real HTTP API, the real queue, and the real
// consumer, so it captures the MQ hop, Redis lock, job creation, polling, and
// download that BenchmarkRealSingle excludes.
//
// Run with -benchtime=1x: each sub-benchmark performs one full export.
func BenchmarkE2ESingle(b *testing.B) {
	for _, n := range []int{100_000, 500_000, 1_000_000} {
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			runE2EBench(b, n, 1_000_000_000)
		})
	}
}

// BenchmarkE2EBatched measures the same end-to-end path through the batched
// (ZIP) export path by forcing max_single_file_rows below the seeded count.
func BenchmarkE2EBatched(b *testing.B) {
	for _, n := range []int{100_000, 500_000, 1_000_000} {
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			runE2EBench(b, n, 1)
		})
	}
}

func runE2EBench(b *testing.B, n, maxSingleFileRows int) {
	b.Helper()

	baseURL, shopID, start, end := setupE2EStack(b, n, maxSingleFileRows)

	b.ReportMetric(float64(n), "rows/op")
	b.ResetTimer()

	var requestSum, processSum, downloadSum time.Duration
	for i := 0; i < b.N; i++ {
		request, process, download := runE2EFlowOnce(b, baseURL, shopID, start, end)
		requestSum += request
		processSum += process
		downloadSum += download
	}

	b.ReportMetric(float64(requestSum)/float64(b.N), "request_ns/op")
	b.ReportMetric(float64(processSum)/float64(b.N), "process_ns/op")
	b.ReportMetric(float64(downloadSum)/float64(b.N), "download_ns/op")
	b.ReportMetric(float64(n)*float64(b.N)/b.Elapsed().Seconds(), "rows_per_sec")
}

// setupE2EStack wires the full stack — real MySQL/MinIO/Redis plus a real
// RocketMQ producer, consumer, and Hertz API server — and returns the API base
// URL and the seeded export range.
func setupE2EStack(b *testing.B, n, maxSingleFileRows int) (string, int64, time.Time, time.Time) {
	b.Helper()

	ctx := context.Background()
	d := setupDeps(b)

	redisClient, err := commonredis.New(d.infra.redis)
	if err != nil {
		b.Fatalf("init redis: %v", err)
	}
	b.Cleanup(func() { _ = redisClient.Close() })

	redisRepo, err := redisrepository.New(redisClient)
	if err != nil {
		b.Fatalf("redis repo: %v", err)
	}

	producerManager, err := rocketmqproducer.New(appinfo.Default())
	if err != nil {
		b.Fatalf("init mq producer: %v", err)
	}
	if err := producerManager.Register(rocketmqproducer.Config{
		Endpoints: d.infra.namesrv,
		Topic:     string(constant.MQTopicReporting),
	}); err != nil {
		b.Fatalf("register mq producer: %v", err)
	}
	if err := producerManager.Start(); err != nil {
		b.Fatalf("start mq producer: %v", err)
	}
	b.Cleanup(func() { _ = producerManager.Shutdown() })

	mqRepo, err := mqrepository.New(producerManager)
	if err != nil {
		b.Fatalf("mq repo: %v", err)
	}

	dl := newDynamicLoader(b, maxSingleFileRows, constant.DefaultMaxBatchPipelineWorkers)
	b.Cleanup(func() { _ = dl.Stop() })

	uc, err := reportusecase.New(d.mysqlRepo, mqRepo, redisRepo, d.s3Repo, dl)
	if err != nil {
		b.Fatalf("report use case: %v", err)
	}

	shopID := benchShopIDFor(n)
	start := benchStart
	end := benchStart.Add(benchWindow)
	if err := seedBenchRows(ctx, d.rawDB, shopID, n, start, end); err != nil {
		b.Fatalf("seed report rows: %v", err)
	}

	// Wire the consumer before publishing, so the message is not missed while
	// the subscription is still being established.
	consumerCfg := d.infra.mq
	consumerCfg.Group = fmt.Sprintf("e2e_bench_%d", time.Now().UnixNano())
	consumerCfg.ConsumeFromWhere = rocketmqconsumer.ConsumeFromLast

	mqHandler, err := mqhandler.New(uc)
	if err != nil {
		b.Fatalf("mq handler: %v", err)
	}

	consumerManager, err := rocketmqconsumer.New(appinfo.Default())
	if err != nil {
		b.Fatalf("init mq consumer: %v", err)
	}
	if err := consumerManager.Register(consumerCfg, mqHandler.ProcessExportReport); err != nil {
		b.Fatalf("register mq consumer: %v", err)
	}
	if err := consumerManager.Start(); err != nil {
		b.Fatalf("start mq consumer: %v", err)
	}
	b.Cleanup(func() { _ = consumerManager.Shutdown() })

	// Allow the consumer to finish rebalancing before publishing.
	time.Sleep(2 * time.Second)

	return "http://" + startAPIServer(b, uc), shopID, start, end
}

// runE2EFlowOnce performs one full asynchronous export through the HTTP API and
// downloads the produced file, returning the request, poll-until-success, and
// download durations separately.
func runE2EFlowOnce(b *testing.B, baseURL string, shopID int64, start, end time.Time) (request, process, download time.Duration) {
	b.Helper()

	// 1. Request the export.
	reqStart := time.Now()
	var exportResp apimodel.ExportReportResponse
	status := httpDoJSON(b, http.MethodPost, baseURL+constant.RouteExportReport, apimodel.ExportReportRequest{
		RequestID: strconv.FormatInt(time.Now().UnixNano(), 10),
		ShopID:    strconv.FormatInt(shopID, 10),
		StartTime: start.Format(time.RFC3339),
		EndTime:   end.Format(time.RFC3339),
	}, &exportResp)
	if status != http.StatusOK {
		b.Fatalf("POST export returned %d", status)
	}
	if exportResp.Data == nil || exportResp.Data.JobID == "" {
		b.Fatalf("expected a job_id, got %+v", exportResp)
	}
	request = time.Since(reqStart)

	// 2. Poll until the job reaches a terminal state.
	procStart := time.Now()
	deadline := time.Now().Add(120 * time.Second)
	jobURL := fmt.Sprintf("%s%s/%s", baseURL, constant.RouteExportReport, exportResp.Data.JobID)
	var jobData apimodel.GetExportReportJobData
	for time.Now().Before(deadline) {
		var jobResp apimodel.GetExportReportJobResponse
		status := httpDoJSON(b, http.MethodGet, jobURL, nil, &jobResp)
		if status != http.StatusOK {
			b.Fatalf("GET job returned %d", status)
		}
		if jobResp.Data == nil {
			b.Fatalf("expected job data, got %+v", jobResp)
		}
		jobData = *jobResp.Data
		if jobData.Status == string(constant.ExportReportJobStatusSuccess) || jobData.Status == string(constant.ExportReportJobStatusFailed) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	process = time.Since(procStart)

	if jobData.Status != string(constant.ExportReportJobStatusSuccess) {
		b.Fatalf("expected job success, got %s (err=%s)", jobData.Status, jobData.ErrorMessage)
	}
	if jobData.DownloadURL == "" {
		b.Fatal("expected a presigned download URL on success")
	}

	// 3. Download the report file (streamed to discard; bytes are not retained).
	dlStart := time.Now()
	resp, err := http.Get(jobData.DownloadURL)
	if err != nil {
		b.Fatalf("download report: %v", err)
	}
	n, copyErr := io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b.Fatalf("download report returned %d", resp.StatusCode)
	}
	if copyErr != nil {
		b.Fatalf("read report: %v", copyErr)
	}
	if n == 0 {
		b.Fatal("downloaded an empty report file")
	}
	download = time.Since(dlStart)

	return request, process, download
}
