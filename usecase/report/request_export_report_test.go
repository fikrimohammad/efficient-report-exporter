package report

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
)

func TestRequestExportReport_CreatesNewJob(t *testing.T) {
	mysql := defaultMockMySQL()
	mq := defaultMockMQ()
	redis := defaultMockRedis()
	s3 := defaultMockS3()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mu := &useCase{
		mySQLRepository: mysql,
		mqRepository:    mq,
		redisRepository: redis,
		s3Repository:    s3,
		dynamicConfig:   dl,
	}

	mysql.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return nil, nil
	}
	mysql.queryReportFn = func(_ context.Context, _ repository.QueryReportFilter) ([]*model.Report, error) {
		return []*model.Report{{ID: 1}}, nil
	}

	var insertedParams repository.InsertExportReportJobParams
	mysql.insertExportReportJob = func(_ context.Context, params repository.InsertExportReportJobParams) (*model.ExportReportJob, error) {
		insertedParams = params
		return &model.ExportReportJob{ID: 42, Status: constant.ExportReportJobStatusProcessing}, nil
	}

	var publishedMsg model.ExportReportProcessMessage
	mq.publishProcessMsg = func(_ context.Context, msg model.ExportReportProcessMessage) error {
		publishedMsg = msg
		return nil
	}

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := mu.RequestExportReport(context.Background(), usecase.RequestExportReportParams{
		RequestID: 100,
		ShopID:    200,
		StartTime: now,
		EndTime:   now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("RequestExportReport failed: %v", err)
	}
	if result.JobID != 42 {
		t.Fatalf("expected JobID 42, got %d", result.JobID)
	}
	if insertedParams.ShopID != 200 {
		t.Fatalf("expected ShopID 200, got %d", insertedParams.ShopID)
	}
	if insertedParams.RequestID != 100 {
		t.Fatalf("expected RequestID 100, got %d", insertedParams.RequestID)
	}
	if publishedMsg.JobID != 42 {
		t.Fatalf("expected published JobID 42, got %d", publishedMsg.JobID)
	}
}

func TestRequestExportReport_ExistingJobAlreadySuccess(t *testing.T) {
	mysql := defaultMockMySQL()
	mq := defaultMockMQ()
	redis := defaultMockRedis()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mysql.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return []*model.ExportReportJob{{ID: 99, Status: constant.ExportReportJobStatusSuccess}}, nil
	}
	mysql.queryReportFn = func(_ context.Context, _ repository.QueryReportFilter) ([]*model.Report, error) {
		return []*model.Report{{ID: 1}}, nil
	}

	var published bool
	mq.publishProcessMsg = func(_ context.Context, _ model.ExportReportProcessMessage) error {
		published = true
		return nil
	}

	mu := &useCase{
		mySQLRepository: mysql,
		mqRepository:    mq,
		redisRepository: redis,
		s3Repository:    defaultMockS3(),
		dynamicConfig:   dl,
	}

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := mu.RequestExportReport(context.Background(), usecase.RequestExportReportParams{
		RequestID: 100,
		ShopID:    200,
		StartTime: now,
		EndTime:   now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result.JobID != 99 {
		t.Fatalf("expected JobID 99 for existing success job, got %d", result.JobID)
	}
	if published {
		t.Fatal("should not publish process message for already-successful job")
	}
}

func TestRequestExportReport_ExistingJobProcessing(t *testing.T) {
	mysql := defaultMockMySQL()
	mq := defaultMockMQ()
	redis := defaultMockRedis()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mysql.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return []*model.ExportReportJob{{ID: 55, Status: constant.ExportReportJobStatusProcessing}}, nil
	}
	mysql.queryReportFn = func(_ context.Context, _ repository.QueryReportFilter) ([]*model.Report, error) {
		return []*model.Report{{ID: 1}}, nil
	}

	var publishedProcess bool
	mq.publishProcessMsg = func(_ context.Context, _ model.ExportReportProcessMessage) error {
		publishedProcess = true
		return nil
	}

	mu := &useCase{
		mySQLRepository: mysql,
		mqRepository:    mq,
		redisRepository: redis,
		s3Repository:    defaultMockS3(),
		dynamicConfig:   dl,
	}

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := mu.RequestExportReport(context.Background(), usecase.RequestExportReportParams{
		RequestID: 100,
		ShopID:    200,
		StartTime: now,
		EndTime:   now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result.JobID != 55 {
		t.Fatalf("expected JobID 55, got %d", result.JobID)
	}
	if publishedProcess {
		t.Fatal("should not publish process message for already-processing job")
	}
}

func TestRequestExportReport_ValidationError(t *testing.T) {
	mu := &useCase{}
	_, err := mu.RequestExportReport(context.Background(), usecase.RequestExportReportParams{
		ShopID:    0, // invalid: zero shop_id
		StartTime: time.Now(),
		EndTime:   time.Now(),
	})
	if err == nil {
		t.Fatal("expected validation error for missing shop_id")
	}
}

func TestRequestExportReport_LockAlreadyHeld(t *testing.T) {
	mysql := defaultMockMySQL()
	mq := defaultMockMQ()
	redis := defaultMockRedis()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	redis.lockRequestFn = func(_ context.Context, _ repository.LockExportReportRequest) error {
		return errors.New("lock is already locked")
	}

	mu := &useCase{
		mySQLRepository: mysql,
		mqRepository:    mq,
		redisRepository: redis,
		s3Repository:    defaultMockS3(),
		dynamicConfig:   dl,
	}

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := mu.RequestExportReport(context.Background(), usecase.RequestExportReportParams{
		RequestID: 100,
		ShopID:    200,
		StartTime: now,
		EndTime:   now.Add(24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected error when lock is held")
	}
}

func TestRequestExportReport_NoReportData(t *testing.T) {
	mysql := defaultMockMySQL()
	redis := defaultMockRedis()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mysql.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return nil, nil
	}
	mysql.queryReportFn = func(_ context.Context, _ repository.QueryReportFilter) ([]*model.Report, error) {
		return nil, nil // no data
	}

	mu := &useCase{
		mySQLRepository: mysql,
		mqRepository:    defaultMockMQ(),
		redisRepository: redis,
		s3Repository:    defaultMockS3(),
		dynamicConfig:   dl,
	}

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := mu.RequestExportReport(context.Background(), usecase.RequestExportReportParams{
		RequestID: 100,
		ShopID:    200,
		StartTime: now,
		EndTime:   now.Add(24 * time.Hour),
	})
	if err == nil || err.Error() != "report data not found" {
		t.Fatalf("expected 'report data not found' error, got: %v", err)
	}
}

func TestRequestExportReport_LockError(t *testing.T) {
	mysql := defaultMockMySQL()
	redis := defaultMockRedis()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	redis.lockRequestFn = func(_ context.Context, _ repository.LockExportReportRequest) error {
		return errors.New("redis down")
	}

	mu := &useCase{
		mySQLRepository: mysql,
		mqRepository:    defaultMockMQ(),
		redisRepository: redis,
		s3Repository:    defaultMockS3(),
		dynamicConfig:   dl,
	}

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := mu.RequestExportReport(context.Background(), usecase.RequestExportReportParams{
		RequestID: 100,
		ShopID:    200,
		StartTime: now,
		EndTime:   now.Add(24 * time.Hour),
	})
	if err == nil || err.Error() != "redis down" {
		t.Fatalf("expected 'redis down' error, got: %v", err)
	}
}

func TestRequestExportReport_JobQueryError(t *testing.T) {
	mysql := defaultMockMySQL()
	redis := defaultMockRedis()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mysql.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return nil, errors.New("db error")
	}

	mu := &useCase{
		mySQLRepository: mysql,
		mqRepository:    defaultMockMQ(),
		redisRepository: redis,
		s3Repository:    defaultMockS3(),
		dynamicConfig:   dl,
	}

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := mu.RequestExportReport(context.Background(), usecase.RequestExportReportParams{
		RequestID: 100,
		ShopID:    200,
		StartTime: now,
		EndTime:   now.Add(24 * time.Hour),
	})
	if err == nil || err.Error() != "db error" {
		t.Fatalf("expected 'db error', got: %v", err)
	}
}

func TestRequestExportReport_DataExistanceQueryError(t *testing.T) {
	mysql := defaultMockMySQL()
	redis := defaultMockRedis()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mysql.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return nil, nil
	}
	mysql.queryReportFn = func(_ context.Context, _ repository.QueryReportFilter) ([]*model.Report, error) {
		return nil, errors.New("db error on data check")
	}

	mu := &useCase{
		mySQLRepository: mysql,
		mqRepository:    defaultMockMQ(),
		redisRepository: redis,
		s3Repository:    defaultMockS3(),
		dynamicConfig:   dl,
	}

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := mu.RequestExportReport(context.Background(), usecase.RequestExportReportParams{
		RequestID: 100,
		ShopID:    200,
		StartTime: now,
		EndTime:   now.Add(24 * time.Hour),
	})
	if err == nil || err.Error() != "db error on data check" {
		t.Fatalf("expected 'db error on data check', got: %v", err)
	}
}

func TestRequestExportReport_PublishProcessMsgError(t *testing.T) {
	mysql := defaultMockMySQL()
	mq := defaultMockMQ()
	redis := defaultMockRedis()
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mysql.queryExportReportJob = func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
		return nil, nil
	}
	mysql.queryReportFn = func(_ context.Context, _ repository.QueryReportFilter) ([]*model.Report, error) {
		return []*model.Report{{ID: 1}}, nil
	}
	mysql.insertExportReportJob = func(_ context.Context, _ repository.InsertExportReportJobParams) (*model.ExportReportJob, error) {
		return &model.ExportReportJob{ID: 42, Status: constant.ExportReportJobStatusProcessing}, nil
	}

	mq.publishProcessMsg = func(_ context.Context, _ model.ExportReportProcessMessage) error {
		return errors.New("mq publish error")
	}

	mu := &useCase{
		mySQLRepository: mysql,
		mqRepository:    mq,
		redisRepository: redis,
		s3Repository:    defaultMockS3(),
		dynamicConfig:   dl,
	}

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := mu.RequestExportReport(context.Background(), usecase.RequestExportReportParams{
		RequestID: 100,
		ShopID:    200,
		StartTime: now,
		EndTime:   now.Add(24 * time.Hour),
	})
	if err == nil || err.Error() != "mq publish error" {
		t.Fatalf("expected 'mq publish error', got: %v", err)
	}
}
