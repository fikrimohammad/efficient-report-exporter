package report

import (
	"github.com/fikrimohammad/efficient-report-exporter/common/confloader"
	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	"github.com/fikrimohammad/efficient-report-exporter/config"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
)

type useCase struct {
	mySQLRepository repository.MySQL
	mqRepository    repository.MQ
	redisRepository repository.Redis
	s3Repository    repository.S3
	dynamicConfig   *confloader.Loader[config.DynamicConfig]
}

func New(mySQLRepository repository.MySQL, mqRepository repository.MQ, redisRepository repository.Redis, s3Repository repository.S3, dynamicConfig *confloader.Loader[config.DynamicConfig]) (usecase.Report, error) {
	if mySQLRepository == nil {
		return nil, errs.New(errs.Internal, "mysql repository is not initialized")
	}

	if mqRepository == nil {
		return nil, errs.New(errs.Internal, "mq repository is not initialized")
	}

	if redisRepository == nil {
		return nil, errs.New(errs.Internal, "redis repository is not initialized")
	}

	if s3Repository == nil {
		return nil, errs.New(errs.Internal, "s3 repository is not initialized")
	}

	return &useCase{
		mySQLRepository: mySQLRepository,
		mqRepository:    mqRepository,
		redisRepository: redisRepository,
		s3Repository:    s3Repository,
		dynamicConfig:   dynamicConfig,
	}, nil
}
