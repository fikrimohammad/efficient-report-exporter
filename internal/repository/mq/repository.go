package mq

import (
	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
	rocketmqproducer "github.com/fikrimohammad/go-dev-sdk/rocketmq/producer"
)

type repo struct {
	producer rocketmqproducer.Client
}

func New(producer rocketmqproducer.Client) (repository.MQ, error) {
	if producer == nil {
		return nil, errs.New(constant.Internal, "producer is not initialized")
	}

	return &repo{
		producer: producer,
	}, nil
}
