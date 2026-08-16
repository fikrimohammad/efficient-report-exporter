package app

import (
	"context"
	"fmt"

	"hash/fnv"
	"os"

	"github.com/fikrimohammad/efficient-report-exporter/internal/config"
	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	mqrepository "github.com/fikrimohammad/efficient-report-exporter/internal/repository/mq"
	mysqlrepository "github.com/fikrimohammad/efficient-report-exporter/internal/repository/mysql"
	redisrepository "github.com/fikrimohammad/efficient-report-exporter/internal/repository/redis"
	s3repository "github.com/fikrimohammad/efficient-report-exporter/internal/repository/s3"
	"github.com/fikrimohammad/efficient-report-exporter/internal/usecase"
	reportusecase "github.com/fikrimohammad/efficient-report-exporter/internal/usecase/report"
	"github.com/fikrimohammad/go-dev-sdk/appinfo"
	"github.com/fikrimohammad/go-dev-sdk/db"
	"github.com/fikrimohammad/go-dev-sdk/observability/logs"
	"github.com/fikrimohammad/go-dev-sdk/observability/metrics"
	"github.com/fikrimohammad/go-dev-sdk/observability/tracer"
	commonredis "github.com/fikrimohammad/go-dev-sdk/redis"
	rocketmqproducer "github.com/fikrimohammad/go-dev-sdk/rocketmq/producer"
	commons3 "github.com/fikrimohammad/go-dev-sdk/s3"
	_ "github.com/go-sql-driver/mysql"
	snowflake "github.com/godruoyi/go-snowflake"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type Resource struct {
	Config        *config.AppConfig
	MetricsClient metrics.Client
	TracerClient  tracer.Client
	ReportUseCase usecase.Report

	mqProducerClient *rocketmqproducer.Producer
	redisClient      commonredis.Client
	s3Client         commons3.Client
	dbClient         db.DB

	mysqlRepository repository.MySQL
	s3Repository    repository.S3
	mqRepository    repository.MQ
	redisRepository repository.Redis
}

func NewResource() (resource *Resource, err error) {
	if err := initLogs(); err != nil {
		return nil, err
	}

	cfg, err := config.Load(context.Background())
	if err != nil {
		return nil, err
	}

	resource = &Resource{Config: cfg}

	if err := resource.initMetrics(); err != nil {
		return nil, err
	}

	if err := resource.initTrace(); err != nil {
		return nil, err
	}

	if err := resource.initDB(); err != nil {
		return nil, err
	}

	if err := resource.initRedis(); err != nil {
		return nil, err
	}

	if err := resource.initS3(); err != nil {
		return nil, err
	}

	if err := resource.initMQProducer(); err != nil {
		return nil, err
	}

	if err := resource.initIDGenerator(); err != nil {
		return nil, err
	}

	if err := resource.initRepositories(); err != nil {
		return nil, err
	}

	if err := resource.initUseCases(); err != nil {
		return nil, err
	}

	return resource, nil
}

// initLogs configures the global logger from environment variables.
//
// Supported env vars:
//
//	LOG_FORMAT  — "text" (default) or "json"
//	LOG_LEVEL   — "debug" (default), "info", "warn", or "error"
//
// Records are routed by severity: debug/info to stdout, warn/error to stderr.
func initLogs() error {
	cfg := logs.Config{
		Format: os.Getenv("LOG_FORMAT"),
		Level:  os.Getenv("LOG_LEVEL"),
	}

	log, err := logs.New(appinfo.Default(), cfg)
	if err != nil {
		return err
	}

	logs.SetDefault(log)
	return nil
}

func (r *Resource) initDB() error {
	if r.Config == nil {
		return fmt.Errorf("config is not initialized")
	}

	dbClient, err := db.Connect(
		r.Config.DB,
		db.WithMetrics(r.MetricsClient),
		db.WithTracer(r.TracerClient),
	)
	if err != nil {
		return err
	}

	r.dbClient = dbClient
	return nil
}

func (r *Resource) initRedis() error {
	if r.Config == nil {
		return fmt.Errorf("config is not initialized")
	}

	redisClient, err := commonredis.New(r.Config.Redis, commonredis.WithMetrics(r.MetricsClient), commonredis.WithTracer(r.TracerClient))
	if err != nil {
		return err
	}

	r.redisClient = redisClient
	return nil
}

func (r *Resource) initS3() error {
	if r.Config == nil {
		return fmt.Errorf("config is not initialized")
	}

	s3Client, err := commons3.New(r.Config.S3, commons3.WithMetrics(r.MetricsClient), commons3.WithTracer(r.TracerClient))
	if err != nil {
		return err
	}

	r.s3Client = s3Client
	return nil
}

func (r *Resource) initMQProducer() error {
	if r.Config == nil {
		return fmt.Errorf("config is not initialized")
	}

	producerManager, err := rocketmqproducer.New(appinfo.Default(),
		rocketmqproducer.WithMetrics(r.MetricsClient),
		rocketmqproducer.WithTracer(r.TracerClient),
	)
	if err != nil {
		return err
	}

	for _, producerConfig := range r.Config.MQProducers {
		if err := producerManager.Register(producerConfig); err != nil {
			return err
		}
	}

	if err := producerManager.Start(); err != nil {
		return err
	}

	r.mqProducerClient = producerManager
	return nil
}

func (r *Resource) initIDGenerator() error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname for snowflake worker id: %w", err)
	}
	h := fnv.New32a()
	h.Write([]byte(hostname))
	// Snowflake reserves 10 bits for the machine ID, so shift the 32-bit
	// hostname hash right by 22 to keep only those 10 bits (values 0..1023).
	snowflake.SetMachineID(uint16(h.Sum32() >> 22))
	snowflake.SetStartTime(constant.SnowflakeEpoch)
	return nil
}

func (r *Resource) initRepositories() error {
	if r.dbClient == nil {
		return fmt.Errorf("database client is not initialized")
	}

	mysqlRepository, err := mysqlrepository.New(r.dbClient)
	if err != nil {
		return err
	}

	s3Repository, err := s3repository.New(r.s3Client)
	if err != nil {
		return err
	}

	mqRepository, err := mqrepository.New(r.mqProducerClient)
	if err != nil {
		return err
	}

	redisRepository, err := redisrepository.New(r.redisClient)
	if err != nil {
		return err
	}

	r.mysqlRepository = mysqlRepository
	r.s3Repository = s3Repository
	r.mqRepository = mqRepository
	r.redisRepository = redisRepository
	return nil
}

func (r *Resource) initUseCases() error {
	if r.mysqlRepository == nil || r.mqRepository == nil || r.redisRepository == nil || r.s3Repository == nil {
		return fmt.Errorf("repositories are not initialized")
	}

	reportUseCase, err := reportusecase.New(
		r.mysqlRepository,
		r.mqRepository,
		r.redisRepository,
		r.s3Repository,
		r.Config.Dynamic,
	)
	if err != nil {
		return err
	}

	r.ReportUseCase = reportUseCase
	return nil
}

func (r *Resource) initMetrics() error {
	client, err := metrics.New(context.Background(), appinfo.Default(), r.Config.Metrics)
	if err != nil {
		return err
	}

	metrics.SetDefault(client)
	r.MetricsClient = client
	return nil
}

func (r *Resource) initTrace() error {
	client, err := tracer.New(context.Background(), appinfo.Default(), r.Config.Tracer)
	if err != nil {
		return err
	}

	otel.SetTracerProvider(client.Provider())
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer.SetDefault(client)
	r.TracerClient = client
	return nil
}

func (r *Resource) Close() error {
	var firstError error

	if r.TracerClient != nil {
		if err := r.TracerClient.Stop(context.Background()); err != nil {
			firstError = err
		}
	}

	if r.MetricsClient != nil {
		if err := r.MetricsClient.Stop(context.Background()); err != nil && firstError == nil {
			firstError = err
		}
	}

	if r.Config != nil {
		if err := r.Config.Dynamic.Stop(); err != nil && firstError == nil {
			firstError = err
		}
	}

	if r.dbClient != nil {
		if err := r.dbClient.Close(); err != nil && firstError == nil {
			firstError = err
		}
	}

	if err := r.redisClient.Close(); err != nil && firstError == nil {
		firstError = err
	}

	if r.mqProducerClient != nil {
		if err := r.mqProducerClient.Shutdown(); err != nil && firstError == nil {
			firstError = err
		}
	}

	return firstError
}
