package s3

import (
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs"
	commons3 "github.com/fikrimohammad/go-dev-sdk/s3"
)

type repo struct {
	s3 commons3.Client
}

func New(s3 commons3.Client) (repository.S3, error) {
	if s3 == nil {
		return nil, errs.New(errs.Internal, "s3 client is not initialized")
	}

	return &repo{s3: s3}, nil
}
