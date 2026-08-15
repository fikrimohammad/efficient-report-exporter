package report

import (
	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/config"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/fikrimohammad/go-dev-sdk/confloader"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

type useCase struct {
	mysqlRepository repository.MySQL
	mqRepository    repository.MQ
	redisRepository repository.Redis
	s3Repository    repository.S3
	dynamicConfig   *confloader.Loader[config.DynamicConfig]
}

func New(mysqlRepository repository.MySQL, mqRepository repository.MQ, redisRepository repository.Redis, s3Repository repository.S3, dynamicConfig *confloader.Loader[config.DynamicConfig]) (usecase.Report, error) {
	if mysqlRepository == nil {
		return nil, errs.New(apperrors.Internal, "mysql repository is not initialized")
	}

	if mqRepository == nil {
		return nil, errs.New(apperrors.Internal, "mq repository is not initialized")
	}

	if redisRepository == nil {
		return nil, errs.New(apperrors.Internal, "redis repository is not initialized")
	}

	if s3Repository == nil {
		return nil, errs.New(apperrors.Internal, "s3 repository is not initialized")
	}

	return &useCase{
		mysqlRepository: mysqlRepository,
		mqRepository:    mqRepository,
		redisRepository: redisRepository,
		s3Repository:    s3Repository,
		dynamicConfig:   dynamicConfig,
	}, nil
}
