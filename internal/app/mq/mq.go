package mq

import (
	"github.com/fikrimohammad/efficient-report-exporter/internal/app"
	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	mqhandler "github.com/fikrimohammad/efficient-report-exporter/internal/handler/mq"
	"github.com/fikrimohammad/go-dev-sdk/appinfo"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
	"github.com/fikrimohammad/go-dev-sdk/rocketmq/consumer"
)

type Consumer struct {
	manager *consumer.Consumer
}

func New(src *app.Resource) (*Consumer, error) {
	if src == nil {
		return nil, ErrResourceNotInitialized
	}
	if src.ReportUseCase == nil {
		return nil, ErrReportUseCaseNotInitialized
	}

	consumerConfigs := src.Config.MQConsumers
	if len(consumerConfigs) == 0 {
		return nil, ErrConsumerConfigNotInitialized
	}

	manager, err := consumer.New(appinfo.Default(),
		consumer.WithMetrics(src.MetricsClient),
		consumer.WithTracer(src.TracerClient),
	)
	if err != nil {
		return nil, err
	}

	handler, err := mqhandler.New(src.ReportUseCase)
	if err != nil {
		return nil, err
	}

	for _, cfg := range consumerConfigs {
		if err := manager.Register(cfg, handler.ProcessExportReport); err != nil {
			return nil, err
		}
	}

	return &Consumer{manager: manager}, nil
}

func (c *Consumer) Start() error {
	return c.manager.Start()
}

func (c *Consumer) Shutdown() error {
	return c.manager.Shutdown()
}

var (
	ErrResourceNotInitialized       = errs.New(constant.Internal, "resource is not initialized")
	ErrReportUseCaseNotInitialized  = errs.New(constant.Internal, "report use case is not initialized")
	ErrConsumerConfigNotInitialized = errs.New(constant.Internal, "mq consumer config is not initialized")
)
