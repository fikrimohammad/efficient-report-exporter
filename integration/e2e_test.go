//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/constant"
	mqhandler "github.com/fikrimohammad/efficient-report-exporter/handler/mq"
	mqrepository "github.com/fikrimohammad/efficient-report-exporter/repository/mq"
	redisrepository "github.com/fikrimohammad/efficient-report-exporter/repository/redis"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	reportusecase "github.com/fikrimohammad/efficient-report-exporter/usecase/report"
	"github.com/fikrimohammad/go-dev-sdk/appinfo"
	commonredis "github.com/fikrimohammad/go-dev-sdk/redis"
	rocketmqconsumer "github.com/fikrimohammad/go-dev-sdk/rocketmq/consumer"
	rocketmqproducer "github.com/fikrimohammad/go-dev-sdk/rocketmq/producer"
)

// TestEndToEndExportThroughRocketMQ exercises the full asynchronous flow against
// the real queue: RequestExportReport publishes a process message to RocketMQ,
// a real consumer consumes it and runs the export pipeline, and the job then
// reports success with a presigned download URL.
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

	result, err := uc.RequestExportReport(ctx, usecase.RequestExportReportParams{
		RequestID: time.Now().UnixNano(),
		ShopID:    shopID,
		StartTime: start,
		EndTime:   end,
	})
	if err != nil {
		t.Fatalf("request export: %v", err)
	}

	deadline := time.Now().Add(60 * time.Second)
	var job *usecase.GetExportReportJobResult
	for time.Now().Before(deadline) {
		job, err = uc.GetExportReportJob(ctx, usecase.GetExportReportJobParams{JobID: result.JobID})
		if err != nil {
			t.Fatalf("get export job: %v", err)
		}
		if job.Status == constant.ExportReportJobStatusSuccess || job.Status == constant.ExportReportJobStatusFailed {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if job.Status != constant.ExportReportJobStatusSuccess {
		t.Fatalf("expected job success, got %s (err=%s)", job.Status, job.ErrorMessage)
	}
	if job.DownloadURL == "" {
		t.Fatal("expected a presigned download URL on success")
	}
}
