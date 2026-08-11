package mq

import (
	"github.com/fikrimohammad/go-dev-sdk/errs"
	rocketmqproducer "github.com/fikrimohammad/go-dev-sdk/rocketmq/producer"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

type repo struct {
	producer rocketmqproducer.Client
}

func New(producer rocketmqproducer.Client) (repository.MQ, error) {
	if producer == nil {
		return nil, errs.New(errs.Internal, "producer is not initialized")
	}

	return &repo{
		producer: producer,
	}, nil
}
