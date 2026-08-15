package report

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/mocks"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"go.uber.org/mock/gomock"
)

func TestRequestExportReport_CreatesNewJob(t *testing.T) {
	ctrl := gomock.NewController(t)
	mysql := mocks.NewMockMySQL(ctrl)
	mq := mocks.NewMockMQ(ctrl)
	redis := mocks.NewMockRedis(ctrl)
	allowRedisLocks(redis)

	mysql.EXPECT().QueryExportReportJob(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mysql.EXPECT().QueryReport(gomock.Any(), gomock.Any()).Return([]*model.Report{{ID: 1}}, nil).AnyTimes()

	var insertedParams repository.InsertExportReportJobParams
	mysql.EXPECT().
		InsertExportReportJob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, params repository.InsertExportReportJobParams) (*model.ExportReportJob, error) {
			insertedParams = params
			return &model.ExportReportJob{ID: 42, Status: constant.ExportReportJobStatusProcessing}, nil
		}).
		AnyTimes()

	var publishedMsg model.ExportReportProcessMessage
	mq.EXPECT().
		PublishExportReportProcessMsg(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, msg model.ExportReportProcessMessage) error {
			publishedMsg = msg
			return nil
		}).
		AnyTimes()

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mu := &useCase{
		mysqlRepository: mysql,
		mqRepository:    mq,
		redisRepository: redis,
		s3Repository:    mocks.NewMockS3(ctrl),
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
	ctrl := gomock.NewController(t)
	mysql := mocks.NewMockMySQL(ctrl)
	mq := mocks.NewMockMQ(ctrl)
	redis := mocks.NewMockRedis(ctrl)
	allowRedisLocks(redis)

	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		Return([]*model.ExportReportJob{{ID: 99, Status: constant.ExportReportJobStatusSuccess}}, nil).
		AnyTimes()

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mu := &useCase{
		mysqlRepository: mysql,
		mqRepository:    mq,
		redisRepository: redis,
		s3Repository:    mocks.NewMockS3(ctrl),
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
}

func TestRequestExportReport_ExistingJobProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	mysql := mocks.NewMockMySQL(ctrl)
	mq := mocks.NewMockMQ(ctrl)
	redis := mocks.NewMockRedis(ctrl)
	allowRedisLocks(redis)

	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		Return([]*model.ExportReportJob{{ID: 55, Status: constant.ExportReportJobStatusProcessing}}, nil).
		AnyTimes()

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mu := &useCase{
		mysqlRepository: mysql,
		mqRepository:    mq,
		redisRepository: redis,
		s3Repository:    mocks.NewMockS3(ctrl),
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
	ctrl := gomock.NewController(t)
	redis := mocks.NewMockRedis(ctrl)
	redis.EXPECT().
		LockExportReportRequest(gomock.Any(), gomock.Any()).
		Return("", errors.New("lock is already locked")).
		AnyTimes()

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mu := &useCase{
		mysqlRepository: mocks.NewMockMySQL(ctrl),
		mqRepository:    mocks.NewMockMQ(ctrl),
		redisRepository: redis,
		s3Repository:    mocks.NewMockS3(ctrl),
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
	ctrl := gomock.NewController(t)
	mysql := mocks.NewMockMySQL(ctrl)
	redis := mocks.NewMockRedis(ctrl)
	allowRedisLocks(redis)

	mysql.EXPECT().QueryExportReportJob(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mysql.EXPECT().QueryReport(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mu := &useCase{
		mysqlRepository: mysql,
		mqRepository:    mocks.NewMockMQ(ctrl),
		redisRepository: redis,
		s3Repository:    mocks.NewMockS3(ctrl),
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
	ctrl := gomock.NewController(t)
	redis := mocks.NewMockRedis(ctrl)
	redis.EXPECT().
		LockExportReportRequest(gomock.Any(), gomock.Any()).
		Return("", errors.New("redis down")).
		AnyTimes()

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mu := &useCase{
		mysqlRepository: mocks.NewMockMySQL(ctrl),
		mqRepository:    mocks.NewMockMQ(ctrl),
		redisRepository: redis,
		s3Repository:    mocks.NewMockS3(ctrl),
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
	ctrl := gomock.NewController(t)
	mysql := mocks.NewMockMySQL(ctrl)
	redis := mocks.NewMockRedis(ctrl)
	allowRedisLocks(redis)

	mysql.EXPECT().
		QueryExportReportJob(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("db error")).
		AnyTimes()

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mu := &useCase{
		mysqlRepository: mysql,
		mqRepository:    mocks.NewMockMQ(ctrl),
		redisRepository: redis,
		s3Repository:    mocks.NewMockS3(ctrl),
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
	ctrl := gomock.NewController(t)
	mysql := mocks.NewMockMySQL(ctrl)
	redis := mocks.NewMockRedis(ctrl)
	allowRedisLocks(redis)

	mysql.EXPECT().QueryExportReportJob(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mysql.EXPECT().QueryReport(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error on data check")).AnyTimes()

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mu := &useCase{
		mysqlRepository: mysql,
		mqRepository:    mocks.NewMockMQ(ctrl),
		redisRepository: redis,
		s3Repository:    mocks.NewMockS3(ctrl),
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
	ctrl := gomock.NewController(t)
	mysql := mocks.NewMockMySQL(ctrl)
	mq := mocks.NewMockMQ(ctrl)
	redis := mocks.NewMockRedis(ctrl)
	allowRedisLocks(redis)

	mysql.EXPECT().QueryExportReportJob(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mysql.EXPECT().QueryReport(gomock.Any(), gomock.Any()).Return([]*model.Report{{ID: 1}}, nil).AnyTimes()
	mysql.EXPECT().
		InsertExportReportJob(gomock.Any(), gomock.Any()).
		Return(&model.ExportReportJob{ID: 42, Status: constant.ExportReportJobStatusProcessing}, nil).
		AnyTimes()

	mq.EXPECT().
		PublishExportReportProcessMsg(gomock.Any(), gomock.Any()).
		Return(errors.New("mq publish error")).
		AnyTimes()

	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	mu := &useCase{
		mysqlRepository: mysql,
		mqRepository:    mq,
		redisRepository: redis,
		s3Repository:    mocks.NewMockS3(ctrl),
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
