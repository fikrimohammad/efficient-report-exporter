//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
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

// TestEndToEndExportThroughRocketMQ exercises the full asynchronous flow through
// the public HTTP API and the real queue: POST /v1/reports/export publishes a
// process message to RocketMQ, a real consumer consumes it and runs the export
// pipeline, and GET /v1/reports/export/:job_id then reports success with a
// presigned download URL.
func TestEndToEndExportThroughRocketMQ(t *testing.T) {
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

	dl := newDynamicLoader(t, constant.DefaultMaxSingleFileRows, constant.DefaultMaxBatchPipelineWorkers)
	t.Cleanup(func() { _ = dl.Stop() })

	uc, err := reportusecase.New(d.mysqlRepo, mqRepo, redisRepo, d.s3Repo, dl)
	if err != nil {
		t.Fatalf("report use case: %v", err)
	}

	// A dedicated shop so this test never collides with the benchmark data.
	shopID := int64(424299)
	start := benchStart
	end := benchStart.Add(24 * time.Hour)
	if err := seedBenchRows(ctx, d.rawDB, shopID, 100, start, end); err != nil {
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
		ShopID:    strconv.FormatInt(shopID, 10),
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
	status = httpDoJSON(t, http.MethodGet, fmt.Sprintf("%s%s?shop_id=%d", baseURL, constant.RouteExportReportJobs, shopID), nil, &listResp)
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
